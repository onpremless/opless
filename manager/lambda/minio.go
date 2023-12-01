package lambda

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/mholt/archiver/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	api "github.com/onpremless/go-client"
	cutil "github.com/onpremless/opless/common/util"
	"github.com/onpremless/opless/manager/util"
)

var minioCli *minio.Client

const (
	lambdaBucket  = "lambda"
	tmpBucket     = "lambda-tmp"
	runtimeBucket = "runtime"
)

var tmpTTL time.Duration = time.Duration(cutil.GetIntVar("TMP_TTL"))

func init() {
	endpoint := cutil.GetStrVar("MINIO_ENDPOINT")
	accessKeyID := cutil.GetStrVar("MINIO_ACCESS_KEY")
	secretAccessKey := cutil.GetStrVar("MINIO_SECRET_KEY")

	var err error
	minioCli, err = minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
	})

	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	for {
		_, errBucketExists := minioCli.BucketExists(ctx, lambdaBucket)
		if errBucketExists != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		break
	}

	if err = createBucketIfNecessary(ctx, lambdaBucket); err != nil {
		panic(err)
	}

	if err = createBucketIfNecessary(ctx, tmpBucket); err != nil {
		panic(err)
	}

	if err = createBucketIfNecessary(ctx, runtimeBucket); err != nil {
		panic(err)
	}
}

func createBucketIfNecessary(ctx context.Context, bucket string) error {
	exists, errBucketExists := minioCli.BucketExists(ctx, bucket)
	if errBucketExists != nil {
		return errBucketExists
	}

	if exists {
		return nil
	}

	return minioCli.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
}

func UploadTmp(ctx context.Context, file io.Reader) (string, error) {
	id := cutil.UUID()
	_, err := minioCli.PutObject(ctx, tmpBucket, id, file, -1, minio.PutObjectOptions{})

	if err != nil {
		return "", err
	}

	// Remove uploaded file after N minutes after upload
	// TODO: make possible to pock tmp file to reset timeout
	go func() {
		time.Sleep(tmpTTL * time.Second)
		minioCli.RemoveObject(context.Background(), tmpBucket, id, minio.RemoveObjectOptions{})
	}()

	return id, err
}

func BootstrapLambda(ctx context.Context, id string, lambda *api.CreateLambda) error {
	archive, err := minioCli.GetObject(ctx, tmpBucket, lambda.Archive, minio.GetObjectOptions{})
	if err != nil {
		return err
	}
	defer archive.Close()

	tmpDir, err := os.MkdirTemp("", "opless-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := path.Join(tmpDir, lambda.Archive)
	archFile, err := os.OpenFile(archivePath, os.O_CREATE|os.O_RDWR, 0777)
	if err != nil {
		return err
	}
	defer archFile.Close()

	_, err = io.Copy(archFile, archive)
	if err != nil {
		return err
	}

	mime, err := mimetype.DetectFile(archivePath)
	if err != nil {
		return err
	}

	err = os.Rename(archivePath, archivePath+mime.Extension())
	archivePath += mime.Extension()
	if err != nil {
		return err
	}

	dest := path.Join(tmpDir, "extracted")
	err = os.Mkdir(dest, 0777)
	if err != nil {
		return err
	}

	err = archiver.Unarchive(archivePath, dest)
	if err != nil {
		return err
	}

	err = filepath.Walk(dest, func(file string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		_, lerr := minioCli.FPutObject(ctx, lambdaBucket, id+filepath.ToSlash(file)[len(dest):], file, minio.PutObjectOptions{})

		return lerr
	})

	if err != nil {
		return err
	}

	return nil
}

func BootstrapRuntime(ctx context.Context, id string, runtime *api.CreateRuntime) error {
	_, err := minioCli.CopyObject(ctx, minio.CopyDestOptions{
		Bucket:       runtimeBucket,
		Object:       id,
		UserMetadata: map[string]string{"name": runtime.Name}},
		minio.CopySrcOptions{Bucket: tmpBucket, Object: runtime.Dockerfile})
	if err != nil {
		return err
	}

	return nil
}

func TarLambda(ctx context.Context, lambda string, runtime string) (io.Reader, error) {
	objectCh := minioCli.ListObjects(ctx, lambdaBucket, minio.ListObjectsOptions{
		Prefix:    lambda,
		Recursive: true,
	})

	dir, err := os.MkdirTemp("", "opless-lambda-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}

		oPath := object.Key
		aPath := strings.Join(strings.Split(oPath, "/")[1:], "/")
		fileDir := path.Join(dir, path.Dir(aPath))
		if err := os.MkdirAll(fileDir, 0777); err != nil {
			os.RemoveAll(dir)
			return nil, err
		}

		if err := minioCli.FGetObject(ctx, lambdaBucket, oPath, path.Join(dir, aPath), minio.GetObjectOptions{}); err != nil {
			return nil, err
		}
	}

	dockerfilePath := path.Join(dir, "Dockerfile")
	if err := minioCli.FGetObject(ctx, runtimeBucket, runtime, dockerfilePath, minio.GetObjectOptions{}); err != nil {
		return nil, err
	}

	return util.Tar(dir)
}

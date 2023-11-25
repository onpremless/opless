package main

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	api "github.com/onpremless/go-client"
	cutil "github.com/onpremless/opless/common/util"
	"github.com/onpremless/opless/manager/endpoint"
	"github.com/onpremless/opless/manager/lambda"
	"github.com/onpremless/opless/manager/logger"
	"github.com/onpremless/opless/manager/model"
	"github.com/onpremless/opless/manager/task"
)

type Services struct {
	taskSvc     task.TaskService
	lambdaSvc   lambda.LambdaService
	endpointSvc endpoint.EndpointService
}

func makeServices() *Services {
	lSvc, err := lambda.CreateLambdaService()
	if err != nil {
		panic(err)
	}

	eSvc := endpoint.CreateEndpointService(lSvc)
	tSvc := task.CreateTaskService()

	return &Services{
		taskSvc:     tSvc,
		lambdaSvc:   lSvc,
		endpointSvc: eSvc,
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	svcs := makeServices()
	srv, err := StartServer(svcs)
	if err != nil {
		panic(err)
	}

	<-ctx.Done()
	stop()

	logger.L.Info("Shutting down gracefully, press Ctrl+C again to force")

	srvCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(srvCtx); err != nil {
		logger.L.Fatal("Server forced to shutdown", zap.Error(err))
	}

	svcCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	svcs.lambdaSvc.Stop(svcCtx)
}

func StartServer(svcs *Services) (*http.Server, error) {
	r := gin.Default()

	r.POST("/upload", func(c *gin.Context) {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			logger.L.Error("internal server error", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			logger.L.Error("internal server error", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		id, err := lambda.UploadTmp(c, file)
		if err != nil {
			logger.L.Error("internal server error", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"id": id})
	})

	r.GET("/lambda", func(c *gin.Context) {
		lambdas, err := lambda.GetLambdas(c)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, lambdas)
	})

	r.GET("/lambda/:id", func(c *gin.Context) {
		lambda, err := lambda.GetLambda(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, lambda)
	})

	r.POST("/lambda", func(c *gin.Context) {
		cLambda := &api.CreateLambda{}
		err := c.ShouldBind(cLambda)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err = model.ValidateCreateLambda(cLambda)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		lambda, err := svcs.lambdaSvc.BootstrapLambda(c, cLambda)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, lambda)
	})

	r.POST("/lambda/:id/start", func(c *gin.Context) {
		id := cutil.UUID()
		svcs.taskSvc.Add(id)

		go func() {
			ctx := context.TODO()

			if err := svcs.lambdaSvc.Start(ctx, c.Param("id")); err != nil {
				svcs.taskSvc.Failed(id, struct {
					Error string `json:"error"`
				}{Error: err.Error()})
				return
			}

			svcs.taskSvc.Succeeded(id, nil)
		}()

		c.JSON(http.StatusAccepted, gin.H{"task": id})
	})

	r.POST("/lambda/:id/destroy", func(c *gin.Context) {
		id := cutil.UUID()
		svcs.taskSvc.Add(id)

		go func() {
			ctx := context.TODO()

			if err := svcs.lambdaSvc.Destroy(ctx, c.Param("id")); err != nil {
				svcs.taskSvc.Failed(id, struct {
					Error string `json:"error"`
				}{Error: err.Error()})
				return
			}

			svcs.taskSvc.Succeeded(id, nil)
		}()

		c.JSON(http.StatusAccepted, gin.H{"task": id})
	})

	r.GET("/runtime", func(c *gin.Context) {
		runtimes, err := lambda.GetRuntimes(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, runtimes)
	})

	r.GET("/runtime/:id", func(c *gin.Context) {
		runtime, err := lambda.GetRuntime(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, runtime)
	})

	r.POST("/runtime", func(c *gin.Context) {
		cRuntime := &api.CreateRuntime{}
		err := c.ShouldBind(cRuntime)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err = model.ValidateCreateRuntime(cRuntime)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		runtime, err := svcs.lambdaSvc.BootstrapRuntime(c, cRuntime)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, runtime)
	})

	r.GET("/endpoint", func(c *gin.Context) {
		endpoints, err := svcs.endpointSvc.List(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, endpoints)
	})

	r.GET("/endpoint/:id", func(c *gin.Context) {
		endpoint, err := svcs.endpointSvc.Get(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, endpoint)
	})

	r.POST("/endpoint", func(c *gin.Context) {
		req := &api.CreateEndpoint{}
		err := c.ShouldBind(req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err = model.ValidateCreateEndpoint(req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		endpoint, err := svcs.endpointSvc.Create(c, req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, endpoint)
	})

	r.GET("/task/:id", func(c *gin.Context) {
		status := svcs.taskSvc.Get(c.Param("id"))

		if status == nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, task.PrepareStatus(status))
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cutil.GetIntVar("PORT")),
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.L.Error("Failed to start server", zap.Error(err))
		}
	}()

	return srv, nil
}

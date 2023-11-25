package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/onpremless/opless/common/util"
	"github.com/onpremless/opless/router/logger"
	"github.com/onpremless/opless/router/service"
	"go.uber.org/zap"
)

func main() {
	svc, err := service.NewService(context.TODO())
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		logger.L.Info(
			"new request",
			zap.String("url", req.URL.String()),
		)

		defer req.Body.Close()

		ctx := req.Context()
		redirect, err := svc.RedirectURL(ctx, req)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
			return
		}

		redirectURL, err := url.Parse(redirect)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
			return
		}

		req.RequestURI = ""
		req.URL = redirectURL

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
			return
		}

		defer resp.Body.Close()

		resp.Header = w.Header().Clone()
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})
	if err := http.ListenAndServe(fmt.Sprintf(":%d", util.GetIntVar("PORT")), nil); err != nil {
		logger.L.Error(
			"http server error",
			zap.Error(err),
		)
	}

	svc.Stop()
}

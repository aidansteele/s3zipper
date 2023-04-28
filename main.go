package main

import (
	"archive/zip"
	"context"
	"fmt"
	"github.com/aidansteele/s3zipper/types"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithClientLogMode(aws.LogRequest|aws.LogResponse))
	if err != nil {
		panic(fmt.Sprintf("%+v", err))
	}

	h := &handler{
		s3:      s3.NewFromConfig(cfg),
		bucket:  os.Getenv("BUCKET"),
		realLen: os.Getenv("REAL_LENGTH") == "true",
	}

	switch os.Getenv("_HANDLER") {
	case "GetObject":
		lambda.Start(h.handleGetObject)
	case "HeadObject":
		lambda.Start(h.handleHeadObject)
	default:
		panic("unexpected _HANDLER")
	}

}

type handler struct {
	s3      *s3.Client
	bucket  string
	realLen bool
}

func (h *handler) handleHeadObject(ctx context.Context, event *types.HeadObjectInput) (*types.HeadObjectOutput, error) {
	u, _ := url.Parse(event.HeadObjectContext.InputS3Url)
	key := strings.TrimPrefix(u.Path, "/")
	prefix, isZip := strings.CutSuffix(key, ".zip")
	if !isZip {
		return &types.HeadObjectOutput{
			StatusCode:   400,
			ErrorCode:    "NotAZipFile",
			ErrorMessage: "This access point can only be used to download ZIP files",
		}, nil
	}

	p := s3.NewListObjectsV2Paginator(h.s3, &s3.ListObjectsV2Input{Bucket: &h.bucket, Prefix: &prefix})

	// the s3 cli uses the `multipart_threshold` threshold value to send Range requests in parallel.
	// we don't support that. users can change the threshold, or we can just lie about the
	// content-length, which appears to work just fine (even if the CLI reports a funny progress
	// during the download). we give deployers the option of asking for a real value to be calculated.
	// this isn't going to be totally accurate (it's missing the zip archive overhead) but that's fine
	var totalSize int64 = 0
	mostRecentLastMod := time.Time{}

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return &types.HeadObjectOutput{
				StatusCode:   500,
				ErrorCode:    "PaginationError",
				ErrorMessage: fmt.Sprintf("There was an error paginating S3: %v", err),
				Headers:      nil,
			}, nil
		}

		for _, object := range page.Contents {
			totalSize += object.Size
			if object.LastModified.After(mostRecentLastMod) {
				mostRecentLastMod = *object.LastModified
			}
		}
	}

	// see earlier comment about why we fudge this
	if !h.realLen {
		totalSize = 1024
	}

	return &types.HeadObjectOutput{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Length": fmt.Sprintf("%d", totalSize),
			"Last-Modified":  mostRecentLastMod.UTC().Format(http.TimeFormat),
		},
	}, nil
}

func (h *handler) handleGetObject(ctx context.Context, event *types.GetObjectInput) (*types.GetObjectOutput, error) {
	u, _ := url.Parse(event.GetObjectContext.InputS3Url)
	key := strings.TrimPrefix(u.Path, "/")
	prefix, isZip := strings.CutSuffix(key, ".zip")
	if !isZip {
		_, err := h.s3.WriteGetObjectResponse(ctx, &s3.WriteGetObjectResponseInput{
			RequestRoute: &event.GetObjectContext.OutputRoute,
			RequestToken: &event.GetObjectContext.OutputToken,
			StatusCode:   400,
			ErrorCode:    aws.String("NotAZipFile"),
			ErrorMessage: aws.String("This access point can only be used to download ZIP files"),
		})
		if err != nil {
			return nil, fmt.Errorf("writing response: %w", err)
		}

		return &types.GetObjectOutput{StatusCode: 400}, nil
	}

	pr, pw := io.Pipe()

	go func() {
		defer func() {
			if rerr := recover(); rerr != nil {
				pw.CloseWithError(fmt.Errorf("panic: %+v", rerr))
			}
		}()

		err := h.writeZip(ctx, pw, prefix)
		_ = pw.CloseWithError(err)
	}()

	_, err := h.s3.WriteGetObjectResponse(ctx, &s3.WriteGetObjectResponseInput{
		RequestRoute: &event.GetObjectContext.OutputRoute,
		RequestToken: &event.GetObjectContext.OutputToken,
		Body:         pr,
	})
	if err != nil {
		return nil, fmt.Errorf("writing response: %w", err)
	}

	return &types.GetObjectOutput{StatusCode: 200}, nil
}

func (h *handler) writeZip(ctx context.Context, w io.Writer, prefix string) error {
	zw := zip.NewWriter(w)

	p := s3.NewListObjectsV2Paginator(h.s3, &s3.ListObjectsV2Input{Bucket: &h.bucket, Prefix: &prefix})

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("paginating: %w", err)
		}

		for _, object := range page.Contents {
			name := strings.TrimPrefix(*object.Key, prefix+"/")
			lastmod := *object.LastModified

			f, err := zw.CreateHeader(&zip.FileHeader{
				Name:     name,
				Modified: lastmod,
			})
			if err != nil {
				return fmt.Errorf("creating header: %w", err)
			}

			// it's pretending to be a directory (probably created by the web console), there's
			// no need to try to download it.
			if strings.HasSuffix(name, "/") {
				continue
			}

			get, err := h.s3.GetObject(ctx, &s3.GetObjectInput{Bucket: &h.bucket, Key: object.Key})
			if err != nil {
				return fmt.Errorf("getting object: %w", err)
			}

			_, err = io.Copy(f, get.Body)
			if err != nil {
				return fmt.Errorf("copying object body: %w", err)
			}

			err = get.Body.Close()
			if err != nil {
				return fmt.Errorf("closing object body: %w", err)
			}
		}
	}

	err := zw.Close()
	if err != nil {
		return fmt.Errorf("closing zip: %w", err)
	}

	return nil
}

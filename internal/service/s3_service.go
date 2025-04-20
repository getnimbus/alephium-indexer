package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/getnimbus/ultrago/u_logger"
)

func NewS3Service(
	sess *session.Session,
) S3Service {
	return &s3Service{
		sess: sess,
	}
}

type S3Service interface {
	GetClient() *s3.S3
	ListObjects(ctx context.Context, bucketName string, objectKey string) ([]string, error)
	CreateBucket(ctx context.Context, bucketName string) error
	UploadFile(ctx context.Context, bucketName string, objectKey string, uploadFileDir string) error
	FileStreamWriter(ctx context.Context, bucket string, key string, errCh chan<- error) *io.PipeWriter
	DeleteFolder(ctx context.Context, bucketName string, objectKey string) error
}

type s3Service struct {
	sess *session.Session
}

func (svc *s3Service) GetClient() *s3.S3 {
	return s3.New(svc.sess)
}

func (svc *s3Service) ListObjects(ctx context.Context, bucketName string, objectKey string) ([]string, error) {
	ctx, logger := u_logger.GetLogger(ctx)

	client := svc.GetClient()
	var res = make([]string, 0)

	// List objects in the specified folder
	err := client.ListObjectsV2PagesWithContext(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(objectKey),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			res = append(res, *obj.Key)
		}
		return !lastPage
	})

	if err != nil {
		logger.Errorf("Failed to list objects in bucket %s with prefix %s: %v", bucketName, objectKey, err)
		return nil, err
	}
	return res, nil
}

func (svc *s3Service) CreateBucket(ctx context.Context, bucketName string) error {
	ctx, logger := u_logger.GetLogger(ctx)

	_, err := svc.GetClient().CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		logger.Errorf("create bucket %s failed, err=%v", bucketName, err)
		return err
	}
	logger.Infof("create bucket %s successful", bucketName)

	return nil
}

func (svc *s3Service) UploadFile(ctx context.Context, bucketName string, objectKey string, uploadFileDir string) error {
	ctx, logger := u_logger.GetLogger(ctx)

	// Create an uploader with the session and custom options
	uploader := s3manager.NewUploader(svc.sess, func(u *s3manager.Uploader) {
		u.PartSize = 16 * 1024 * 1024 // The minimum/default allowed part size is 5MB, we set this is 16MB
		u.Concurrency = 5             // default is 5
	})

	logger.Infof("loading parquet file to upload...")
	file, err := os.Open(uploadFileDir)
	if err != nil {
		logger.Errorf("cannot open filed %s, err=%v", uploadFileDir, err)
		return err
	}
	defer file.Close()

	logger.Infof("start uploading file...")
	// Upload the file to S3.
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Body:   file,
	})
	if err != nil {
		logger.Errorf("upload file %s to bucket %s failed, err=%v", uploadFileDir, bucketName, err)
		return err
	}
	logger.Infof("success file uploaded to, %s", aws.StringValue(&result.Location))

	return nil
}

func (svc *s3Service) FileStreamWriter(ctx context.Context, bucket string, key string, errCh chan<- error) *io.PipeWriter {
	// Open a pipe.
	pr, pw := io.Pipe()

	// Upload from pr in a separate Go routine.
	go func() {
		uploader := s3manager.NewUploader(svc.sess, func(u *s3manager.Uploader) {
			u.PartSize = 5 * 1024 * 1024 // The minimum/default allowed part size is 5MB
			u.Concurrency = 2            // default is 5
		})

		_, err := uploader.UploadWithContext(
			ctx,
			&s3manager.UploadInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
				Body:   pr,
			})
		errCh <- err
	}()

	return pw
}

func (svc *s3Service) DeleteFolder(ctx context.Context, bucketName string, objectKey string) error {
	ctx, logger := u_logger.GetLogger(ctx)

	client := svc.GetClient()
	var objectsToDelete []*s3.ObjectIdentifier

	// List objects in the specified folder
	err := client.ListObjectsV2PagesWithContext(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(objectKey),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			objectsToDelete = append(objectsToDelete, &s3.ObjectIdentifier{
				Key: obj.Key,
			})
		}
		return !lastPage
	})

	if err != nil {
		logger.Errorf("Failed to list objects in bucket %s with prefix %s: %v", bucketName, objectKey, err)
		return err
	}

	if len(objectsToDelete) == 0 {
		logger.Infof("No objects found in bucket %s with prefix %s", bucketName, objectKey)
		return nil
	}

	logger.Infof("Found %d objects to delete", len(objectsToDelete))

	// Delete objects in batches (AWS limits batch deletion to 1000 objects at a time)
	for i := 0; i < len(objectsToDelete); i += 1000 {
		end := i + 1000
		if end > len(objectsToDelete) {
			end = len(objectsToDelete)
		}

		_, err := client.DeleteObjectsWithContext(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucketName),
			Delete: &s3.Delete{
				Objects: objectsToDelete[i:end],
			},
		})
		if err != nil {
			logger.Errorf("Failed to delete objects in bucket %s with prefix %s: %v", bucketName, objectKey, err)
			return err
		}
	}

	logger.Infof("Successfully deleted all objects in bucket %s with prefix %s", bucketName, objectKey)
	return nil
}

// ParseDateFromPath Parse date to check for delete raw data from bucket folder
func ParseDateFromPath(path string) (time.Time, error) {
	re := regexp.MustCompile(`(\d{4}-\d{2}-\d{2})`)
	matches := re.FindStringSubmatch(path)
	if len(matches) < 2 {
		return time.Time{}, fmt.Errorf("no valid date found in key: %s", path)
	}
	parsedDate, err := time.Parse(time.DateOnly, matches[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse date: %v", err)
	}
	return parsedDate, nil
}

package bufbuild

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/bufbuild/buf/internal/buf/ext/extimage"
	imagev1beta1 "github.com/bufbuild/buf/internal/gen/proto/go/v1/bufbuild/buf/image/v1beta1"
	"github.com/bufbuild/buf/internal/pkg/analysis"
	"github.com/bufbuild/buf/internal/pkg/protodesc"
	"github.com/bufbuild/buf/internal/pkg/storage"
	"github.com/bufbuild/buf/internal/pkg/storage/storageos"
	"github.com/bufbuild/buf/internal/pkg/storage/storagepath"
	"github.com/bufbuild/buf/internal/pkg/util/utilgithub/utilgithubtesting"
	"github.com/bufbuild/buf/internal/pkg/util/utilproto/utilprototesting"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	testGoogleapisCommit = "37c923effe8b002884466074f84bc4e78e6ade62"
)

var (
	testGoogleapisDirPath = filepath.Join("cache", "googleapis")
	testLock              sync.Mutex
)

func TestGoogleapis(t *testing.T) {
	t.Parallel()
	//for _, includeSourceInfo := range []bool{true, false} {
	for _, includeSourceInfo := range []bool{false} {
		t.Run(
			fmt.Sprintf("includeSourceInfo:%v", includeSourceInfo),
			func(t *testing.T) {
				t.Parallel()
				testBuildGoogleapis(t, includeSourceInfo)
			},
		)
	}
}

func TestProtocGoogleapis(t *testing.T) {
	t.Parallel()
	//for _, includeSourceInfo := range []bool{true, false} {
	for _, includeSourceInfo := range []bool{false} {
		t.Run(
			fmt.Sprintf("includeSourceInfo:%v", includeSourceInfo),
			func(t *testing.T) {
				t.Parallel()
				testBuildProtocGoogleapis(t, includeSourceInfo)
			},
		)
	}
}

func TestCompareGoogleapis(t *testing.T) {
	t.Parallel()
	//for _, includeSourceInfo := range []bool{true, false} {
	for _, includeSourceInfo := range []bool{false} {
		t.Run(
			fmt.Sprintf("includeSourceInfo:%v", includeSourceInfo),
			func(t *testing.T) {
				t.Parallel()
				image := testBuildGoogleapis(t, includeSourceInfo)
				fileDescriptorSet, err := extimage.ImageToFileDescriptorSet(image)
				assert.NoError(t, err)
				protocFileDescriptorSet := testBuildProtocGoogleapis(t, includeSourceInfo)
				assertFileDescriptorSetsEqual(t, fileDescriptorSet, protocFileDescriptorSet)
			},
		)
	}
}

func testBuildGoogleapis(t *testing.T, includeSourceInfo bool) *imagev1beta1.Image {
	bucket := testGetBucketGoogleapis(t)
	protoFileSet := testGetProtoFileSetGoogleapis(t, bucket)
	image, annotations := testBuild(t, includeSourceInfo, bucket, protoFileSet)
	assert.NoError(t, bucket.Close())

	assert.Equal(t, 0, len(annotations), annotations)
	assert.Equal(t, 1585, len(image.GetFile()))
	importNames, err := extimage.ImageImportNames(image)
	assert.NoError(t, err)
	assert.Equal(
		t,
		[]string{
			"google/protobuf/any.proto",
			"google/protobuf/api.proto",
			"google/protobuf/descriptor.proto",
			"google/protobuf/duration.proto",
			"google/protobuf/empty.proto",
			"google/protobuf/field_mask.proto",
			"google/protobuf/source_context.proto",
			"google/protobuf/struct.proto",
			"google/protobuf/timestamp.proto",
			"google/protobuf/type.proto",
			"google/protobuf/wrappers.proto",
		},
		importNames,
	)

	imageWithoutImports, err := extimage.ImageWithoutImports(image)
	assert.NoError(t, err)
	assert.Equal(t, 1574, len(imageWithoutImports.GetFile()))
	importNames, err = extimage.ImageImportNames(imageWithoutImports)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(importNames))

	imageWithoutImports, err = extimage.ImageWithoutImports(imageWithoutImports)
	assert.NoError(t, err)
	assert.Equal(t, 1574, len(imageWithoutImports.GetFile()))
	importNames, err = extimage.ImageImportNames(imageWithoutImports)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(importNames))

	imageWithSpecificNames, err := extimage.ImageWithSpecificNames(
		image,
		true,
		"google/protobuf/descriptor.proto",
		"google/protobuf/api.proto",
		"google/../google/type/date.proto",
		"google/foo/nonsense.proto",
	)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(imageWithSpecificNames.GetFile()))
	_, err = extimage.ImageWithSpecificNames(
		image,
		false,
		"google/protobuf/descriptor.proto",
		"google/protobuf/api.proto",
		"google/../google/type/date.proto",
		"google/foo/nonsense.proto",
	)
	assert.Equal(t, errors.New("google/foo/nonsense.proto is not present in the Image"), err)
	importNames, err = extimage.ImageImportNames(imageWithSpecificNames)
	assert.NoError(t, err)
	assert.Equal(
		t,
		[]string{
			"google/protobuf/api.proto",
			"google/protobuf/descriptor.proto",
		},
		importNames,
	)
	imageWithoutImports, err = extimage.ImageWithoutImports(imageWithSpecificNames)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(imageWithoutImports.GetFile()))
	importNames, err = extimage.ImageImportNames(imageWithoutImports)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(importNames))

	assert.Equal(t, 1585, len(image.GetFile()))
	// basic check to make sure there is no error at this scale
	_, err = protodesc.NewFilesUnstable(context.Background(), image.GetFile()...)
	assert.NoError(t, err)
	return image
}

func testBuildProtocGoogleapis(t *testing.T, includeSourceInfo bool) *descriptor.FileDescriptorSet {
	bucket := testGetBucketGoogleapis(t)
	protoFileSet := testGetProtoFileSetGoogleapis(t, bucket)
	fileDescriptorSet := testBuildProtoc(t, includeSourceInfo, testGoogleapisDirPath, protoFileSet)
	assert.NoError(t, bucket.Close())
	assert.Equal(t, 1585, len(fileDescriptorSet.GetFile()))
	return fileDescriptorSet
}

func testGetBucketGoogleapis(t *testing.T) storage.ReadBucket {
	testGetGoogleapis(t)
	bucket, err := storageos.NewReadBucket(testGoogleapisDirPath)
	require.NoError(t, err)
	return bucket
}

func testGetProtoFileSetGoogleapis(t *testing.T, bucket storage.ReadBucket) ProtoFileSet {
	protoFileSet, err := newProvider(zap.NewNop()).GetProtoFileSetForBucket(
		context.Background(),
		bucket,
		nil,
		nil,
	)
	require.NoError(t, err)
	return protoFileSet
}

func testBuild(t *testing.T, includeSourceInfo bool, bucket storage.ReadBucket, protoFileSet ProtoFileSet) (*imagev1beta1.Image, []*analysis.Annotation) {
	image, annotations, err := newRunner(zap.NewNop()).Run(
		context.Background(),
		bucket,
		protoFileSet,
		true,
		includeSourceInfo,
	)
	require.NoError(t, err)
	return image, annotations
}

func testBuildProtoc(t *testing.T, includeSourceInfo bool, baseDirPath string, protoFileSet ProtoFileSet) *descriptor.FileDescriptorSet {
	realFilePaths := protoFileSet.RealFilePaths()
	realFilePathsCopy := make([]string, len(realFilePaths))
	for i, realFilePath := range realFilePaths {
		realFilePathsCopy[i] = storagepath.Unnormalize(storagepath.Join(baseDirPath, realFilePath))
	}
	fileDescriptorSet, err := utilprototesting.GetProtocFileDescriptorSet(
		context.Background(),
		[]string{testGoogleapisDirPath},
		realFilePathsCopy,
		true,
		includeSourceInfo,
	)
	require.NoError(t, err)
	return fileDescriptorSet
}

func testGetGoogleapis(t *testing.T) {
	testLock.Lock()
	defer func() {
		if r := recover(); r != nil {
			testLock.Unlock()
			panic(r)
		}
	}()
	defer testLock.Unlock()

	require.NoError(
		t,
		utilgithubtesting.GetGithubArchive(
			context.Background(),
			testGoogleapisDirPath,
			"googleapis",
			"googleapis",
			testGoogleapisCommit,
		),
	)
}

func assertFileDescriptorSetsEqual(t *testing.T, one *descriptor.FileDescriptorSet, two *descriptor.FileDescriptorSet) {
	// This also has the effect of verifying output order
	diffOne, err := utilprototesting.DiffMessagesJSON(one, two, "protoparse-protoc")
	assert.NoError(t, err)
	assert.Equal(t, "", diffOne, "JSON diff:\n%s", diffOne)
	// Cannot compare others due to unknown field issue
	//diffTwo, err := utilprototesting.DiffMessagesText(one, two, "protoparse-protoc")
	//assert.NoError(t, err)
	//assert.Equal(t, "", diffTwo, "Text diff:\n%s", diffTwo)
	//equal, err := proto.Equal(one, two)
	//assert.NoError(t, err)
	//assert.True(t, equal, "proto.Equal returned false")
}

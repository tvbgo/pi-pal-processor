// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package unpack

import (
	"context"
	"fmt"
	"io"
	"testing"
	"testing/iotest"

	"github.com/golang/mock/gomock"
	"github.com/googlecloudplatform/pi-delivery/pkg/cached"
	"github.com/googlecloudplatform/pi-delivery/pkg/obj"
	mock_obj "github.com/googlecloudplatform/pi-delivery/pkg/obj/mocks"
	"github.com/googlecloudplatform/pi-delivery/pkg/resultset"
	"github.com/googlecloudplatform/pi-delivery/pkg/tests"
	"github.com/googlecloudplatform/pi-delivery/pkg/ycd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnpack_UnpackReader(t *testing.T) {
	testCases := []struct {
		testBytes []byte
		expected  []byte
		radix     int
	}{
		{testDecBytes, testDecExpected, 10},
		{testHexBytes, testHexExpected, 16},
	}
	mockCtrl := gomock.NewController(t)
	ctx := context.Background()

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("Radix %d", tc.radix), func(t *testing.T) {
			t.Parallel()

			testSet := resultset.ResultSet{
				{
					Header: &ycd.Header{
						Radix:       tc.radix,
						TotalDigits: int64(0),
						BlockSize:   int64(len(tc.expected)),
						BlockID:     int64(0),
						Length:      198,
					},
					Name:             "Pi - Dec - Chudnovsky/Pi - Dec - Chudnovsky - 0.ycd",
					FirstDigitOffset: 201,
				},
			}

			bucket := mock_obj.NewMockBucket(mockCtrl)
			obj := mock_obj.NewMockObject(mockCtrl)

			bucket.EXPECT().Object(testSet[0].Name).Return(obj).AnyTimes()

			obj.EXPECT().NewRangeReader(
				gomock.AssignableToTypeOf(ctx),
				gomock.Any(),
				gomock.Any(),
			).DoAndReturn(
				func(ctx context.Context, off, length int64) (io.ReadCloser, error) {
					return tests.NewTestReader(testSet, 0, tc.testBytes, off, length)
				},
			).AnyTimes()

			rr := testSet.NewReader(ctx, bucket)
			require.NotNil(t, rr)
			defer assert.NoError(t, rr.Close())

			reader := NewReader(ctx, cached.NewCachedReader(ctx, rr))
			require.NotNil(t, reader)

			buf := make([]byte, len(tc.expected))

			n, err := reader.Read(nil)
			assert.Zero(t, n)
			assert.NoError(t, err)

			dpw := ycd.DigitsPerWord(tc.radix)

			n, err = reader.Read(buf[:dpw])
			assert.Equal(t, dpw, n)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected[:dpw], buf[:dpw])

			n, err = reader.Read(buf[:1])
			assert.Equal(t, 1, n)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected[dpw:dpw+1], buf[:1])

			n, err = reader.Read(buf[:1])
			assert.Equal(t, 1, n)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected[dpw+1:dpw+2], buf[:1])

			off, err := reader.Seek(0, io.SeekStart)
			assert.Zero(t, off)
			assert.NoError(t, err)

			n, err = reader.ReadAt(buf[:10], 0)
			assert.Equal(t, 10, n)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected[:10], buf[:10])

			buf, err = io.ReadAll(reader)
			assert.Equal(t, len(tc.expected), len(buf))
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, buf)

			off, err = reader.Seek(0, io.SeekStart)
			assert.Zero(t, off)
			assert.NoError(t, err)

			assert.NoError(t, iotest.TestReader(reader, tc.expected))
		})
	}
}

func TestUnpack_ReaderWithBoundaries(t *testing.T) {
	t.Parallel()
	const totalDigits = 100

	testSet := resultset.ResultSet{
		{
			Header: &ycd.Header{
				Radix:       10,
				TotalDigits: int64(0),
				BlockSize:   30,
				BlockID:     0,
				Length:      198,
			},
			Name:             "Pi - Dec - Chudnovsky/Pi - Dec - Chudnovsky - 0.ycd",
			FirstDigitOffset: 201,
		},
		{
			Header: &ycd.Header{
				Radix:       10,
				TotalDigits: int64(0),
				BlockSize:   30,
				BlockID:     1,
				Length:      198,
			},
			Name:             "Pi - Dec - Chudnovsky/Pi - Dec - Chudnovsky - 1.ycd",
			FirstDigitOffset: 201,
		},
		{
			Header: &ycd.Header{
				Radix:       10,
				TotalDigits: int64(0),
				BlockSize:   30,
				BlockID:     2,
				Length:      198,
			},
			Name:             "Pi - Dec - Chudnovsky/Pi - Dec - Chudnovsky - 2.ycd",
			FirstDigitOffset: 201,
		},
		{
			Header: &ycd.Header{
				Radix:       10,
				TotalDigits: int64(totalDigits),
				BlockSize:   30,
				BlockID:     3,
				Length:      198,
			},
			Name:             "Pi - Dec - Chudnovsky/Pi - Dec - Chudnovsky - 3.ycd",
			FirstDigitOffset: 201,
		},
	}
	mockCtrl := gomock.NewController(t)
	ctx := context.Background()

	bucket := mock_obj.NewMockBucket(mockCtrl)

	bucket.EXPECT().Object(gomock.Any()).DoAndReturn(func(name string) obj.Object {
		object := mock_obj.NewMockObject(mockCtrl)

		for k, v := range testSet {
			if v.Name == name {
				object.EXPECT().NewRangeReader(
					gomock.AssignableToTypeOf(ctx),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(
					func(ctx context.Context, off, length int64) (io.ReadCloser, error) {
						return tests.NewTestReader(testSet, k, testDecMultipleBlocks, off, length)
					},
				).AnyTimes()
				break
			}
		}

		return object
	}).AnyTimes()

	rr := testSet.NewReader(ctx, bucket)
	require.NotNil(t, rr)
	defer assert.NoError(t, rr.Close())

	// Can't use the cache here because the data is different.
	reader := NewReader(ctx, rr)
	require.NotNil(t, reader)

	buf := make([]byte, totalDigits)

	n, err := reader.ReadAt(buf, 0)
	assert.NoError(t, err)
	assert.Equal(t, totalDigits, n)
	assert.Equal(t, buf, testDecExpected[:totalDigits])

	n, err = io.ReadFull(reader, buf)
	assert.NoError(t, err)
	assert.Equal(t, totalDigits, n)
	assert.Equal(t, buf, testDecExpected[:totalDigits])

	off, err := reader.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	assert.Zero(t, off)

	assert.NoError(t, iotest.TestReader(reader, testDecExpected[:totalDigits]))
}

var testDecBytes = []byte{
	0x60, 0xe2, 0x3e, 0xb8, 0xae, 0x61, 0xa6, 0x13, 0x23, 0x66, 0x57, 0xf6, 0x84, 0x66, 0xef, 0x56,
	0x2e, 0x09, 0x17, 0x1e, 0xbf, 0xd2, 0x7e, 0x63, 0x8e, 0x22, 0xa2, 0x31, 0xfe, 0xa8, 0x16, 0x83,
	0x43, 0xe1, 0x29, 0xbc, 0x73, 0xf4, 0x7c, 0x0c, 0x82, 0xbe, 0x7f, 0xdf, 0x50, 0x84, 0x16, 0x62,
	0x91, 0x64, 0x23, 0x58, 0x12, 0x4b, 0x8e, 0x2a, 0x30, 0x10, 0x80, 0x6b, 0xda, 0x93, 0x1e, 0x72,
	0x72, 0xd3, 0x19, 0xe6, 0x1c, 0xfb, 0x81, 0x0f, 0xf1, 0x23, 0xfd, 0xa2, 0x74, 0x63, 0x51, 0x48,
	0x49, 0x5c, 0xa1, 0xe4, 0x22, 0x31, 0x3b, 0x4c, 0x00, 0x1e, 0x7e, 0x29, 0x73, 0x70, 0xa1, 0x4e,
	0x53, 0xfe, 0x78, 0x05, 0x68, 0xc6, 0x71, 0x20, 0xb2, 0x0c, 0x34, 0x3f, 0xe4, 0x2f, 0xb1, 0x0c,
	0x91, 0x8f, 0x97, 0x06, 0x97, 0x7a, 0x7e, 0x77, 0xdd, 0xec, 0xce, 0xba, 0xca, 0x37, 0x46, 0x54,
	0xd0, 0x06, 0x4a, 0x0f, 0xbf, 0xf4, 0xbe, 0x78, 0xcb, 0xfd, 0xe1, 0xb8, 0x8e, 0x65, 0x1b, 0x15,
	0xf0, 0xd6, 0x18, 0x3c, 0x06, 0xd9, 0x46, 0x63, 0xbc, 0x62, 0xea, 0x0f, 0xc0, 0x69, 0xb9, 0x0f,
	0xb9, 0xd9, 0x69, 0xbb, 0x51, 0x7a, 0x35, 0x13, 0x0f, 0x94, 0x83, 0xfb, 0xed, 0x48, 0x19, 0x3c,
	0x6d, 0x2d, 0x8c, 0xab, 0x32, 0x22, 0xae, 0x49, 0xde, 0x1f, 0xfe, 0xe3, 0x9a, 0x63, 0xe2, 0x18,
	0x09, 0x5b, 0x6d, 0x76, 0x13, 0xfd, 0xba, 0x34, 0xbd, 0x88, 0x3c, 0x1e, 0xec, 0xa4, 0x2b, 0x49,
	0x08, 0x8c, 0x37, 0xbd, 0x41, 0xb3, 0x0c, 0x1b, 0x17, 0x14, 0x37, 0xb1, 0x13, 0xa6, 0x20, 0x5b,
	0x02, 0x57, 0x2c, 0x30, 0x65, 0x9f, 0x26, 0x84, 0x18, 0xc1, 0x8a, 0x7b, 0xab, 0xb4, 0x18, 0x0d,
	0x18, 0x78, 0xe9, 0x9e, 0x07, 0xbb, 0xaf, 0x28, 0xec, 0x29, 0x7d, 0x41, 0x8b, 0x00, 0xe9, 0x5d,
	0x2b, 0xd2, 0xae, 0xf1, 0x59, 0x13, 0x29, 0x63, 0x33, 0x59, 0x87, 0xb8, 0xef, 0x33, 0x91, 0x2f,
	0xff, 0x40, 0xb4, 0xa5, 0xd8, 0x7b, 0x3f, 0x6d, 0xaa, 0x5f, 0x91, 0xf7, 0x1c, 0x54, 0x9a, 0x2f,
	0xdc, 0x9c, 0x59, 0x0a, 0xcc, 0x6d, 0xf3, 0x6d, 0x76, 0x63, 0xd2, 0x2e, 0xd3, 0xd5, 0xb1, 0x1b,
	0x4e, 0x97, 0x13, 0xeb, 0xf0, 0x21, 0xfd, 0x37, 0x71, 0x90, 0x61, 0xa4, 0x39, 0x6f, 0x0a, 0x6b,
	0xc3, 0x86, 0x25, 0x63, 0x5c, 0x91, 0x63, 0x45, 0x3e, 0x6c, 0xb6, 0x73, 0x65, 0xec, 0xb4, 0x0e,
	0x9b, 0x38, 0x0c, 0xd9, 0xee, 0x83, 0x93, 0x52, 0xc4, 0xd6, 0x72, 0xfd, 0x00, 0xd0, 0xaa, 0x03,
	0x02, 0xb1, 0x84, 0x67, 0x0c, 0xd8, 0xe0, 0x45, 0x53, 0x58, 0x48, 0x2a, 0x7a, 0x7a, 0x6f, 0x00,
	0x06, 0x29, 0x28, 0x16, 0xb2, 0xfc, 0x15, 0x2e, 0xfc, 0x40, 0x8d, 0x7a, 0x28, 0xc0, 0xf1, 0x7e,
	0x5e, 0xe8, 0xce, 0x5c, 0x15, 0xa4, 0xd7, 0x68, 0x82, 0x0e, 0x5b, 0x47, 0x59, 0xf1, 0x49, 0x72,
	0xb6, 0xbe, 0xb3, 0x12, 0x8d, 0x29, 0xc8, 0x19, 0xd7, 0x7c, 0x26, 0x49, 0x0e, 0x34, 0x12, 0x55,
	0x75, 0x11, 0xd8, 0xb5, 0x88, 0x54, 0xca, 0x0c, 0x11, 0xd9, 0x34, 0x11, 0xe8, 0x52, 0xef, 0x63,
	0xff, 0xe0, 0x5d, 0x7f, 0x68, 0xd9, 0xea, 0x81, 0x03, 0xcc, 0x98, 0x93, 0x93, 0x80, 0xb5, 0x02,
	0xb1, 0x80, 0x3b, 0x09, 0x2a, 0x07, 0x20, 0x50, 0xd3, 0x16, 0x4e, 0x42, 0xf6, 0x9b, 0x2a, 0x64,
	0x9b, 0x87, 0x7e, 0x46, 0x8a, 0xa2, 0xe2, 0x67, 0x6f, 0x44, 0xf7, 0x87, 0xf9, 0x17, 0x83, 0x0b,
	0x86, 0x4a, 0xe6, 0x20, 0xea, 0x4a, 0x62, 0x7b, 0x1c, 0x21, 0x5b, 0x0f, 0xae, 0x14, 0x1f, 0x5a,
	0xf8, 0x65, 0x8f, 0x81, 0x18, 0xd5, 0xe1, 0x6a, 0x54, 0x47, 0xbd, 0x15, 0x1e, 0xd2, 0xc8, 0x01,
	0x07, 0x5b, 0x40, 0xd4, 0x8b, 0x7b, 0x69, 0x53, 0xe8, 0xcc, 0x81, 0xee, 0x3d, 0x15, 0x07, 0x56,
	0x00, 0x49, 0x21, 0x42, 0x85, 0xe9, 0x70, 0x23, 0x09, 0xfc, 0x72, 0x3f, 0xe3, 0xb2, 0xf7, 0x41,
	0xe4, 0xb6, 0x75, 0x6f, 0x47, 0x4e, 0xdd, 0x7d, 0x59, 0x72, 0xd3, 0x35, 0xfc, 0xbd, 0x45, 0x89,
	0x57, 0x23, 0x5f, 0xf7, 0x18, 0x75, 0xa0, 0x5b, 0x38, 0xb2, 0xfc, 0xed, 0xad, 0xe8, 0x64, 0x11,
	0x68, 0x60, 0x93, 0xb8, 0x4f, 0xd7, 0x58, 0x22, 0x1b, 0xed, 0x7c, 0xd0, 0xe1, 0x37, 0xc2, 0x64,
	0x29, 0x9c, 0x07, 0xc6, 0x9d, 0xc8, 0x5b, 0x42, 0x8d, 0x36, 0xd2, 0xde, 0x75, 0x4a, 0x85, 0x1e,
	0xaa, 0x23, 0xab, 0x35, 0xe1, 0x7b, 0x26, 0x73, 0xea, 0xf8, 0xb2, 0x4e, 0x35, 0xb3, 0x40, 0x4c,
	0xb1, 0x8a, 0x7c, 0xde, 0x13, 0x25, 0xad, 0x80, 0x8c, 0xdc, 0x14, 0xa7, 0xe9, 0x70, 0x5f, 0x1d,
	0x09, 0xb1, 0x52, 0x8f, 0xe9, 0x25, 0x48, 0x03, 0x9a, 0xf0, 0x2c, 0xd6, 0x56, 0xf9, 0xd9, 0x73,
	0x11, 0xb8, 0xd4, 0xa4, 0xd8, 0xcc, 0xd5, 0x24, 0xf9, 0xe4, 0x82, 0xac, 0xdc, 0xa4, 0x1e, 0x69,
	0xac, 0x98, 0x37, 0x9b, 0x30, 0x9c, 0x08, 0x6f, 0x73, 0x38, 0x28, 0x4a, 0x17, 0x92, 0x44, 0x44,
	0x41, 0x2d, 0xe2, 0xbe, 0xb9, 0x80, 0xf6, 0x01, 0xe5, 0x81, 0xe7, 0x8c, 0xfc, 0xe2, 0x1e, 0x32,
	0x66, 0xdd, 0xb5, 0x4f, 0xfb, 0xea, 0x13, 0x3a, 0xf0, 0xac, 0x0a, 0xda, 0x2f, 0x46, 0x9d, 0x66,
	0xc4, 0xd5, 0xe6, 0x14, 0x75, 0xd4, 0xb2, 0x77, 0xf1, 0x88, 0x69, 0x66, 0xb0, 0xbf, 0xb1, 0x89,
	0x89, 0xe3, 0xcf, 0x95, 0xc8, 0x80, 0x13, 0x3b, 0x47, 0xdb, 0x90, 0x1e, 0xf8, 0xcd, 0xaf, 0x5a,
	0xcc, 0x44, 0x2c, 0x2a, 0xdf, 0xf7, 0x60, 0x7f, 0x1b, 0x3b, 0x22, 0x2e, 0x85, 0xdd, 0xb2, 0x6e,
	0xcb, 0xa8, 0x3c, 0xe9, 0x2e, 0x27, 0x21, 0x6d, 0x08, 0xa5, 0x16, 0x99, 0x12, 0xa8, 0xfa, 0x22,
	0x91, 0x3a, 0x55, 0x54, 0x88, 0x56, 0x4b, 0x39, 0x97, 0x10, 0xb7, 0x4e, 0x22, 0xd0, 0xf5, 0x85,
	0x86, 0x6c, 0x4a, 0x41, 0x91, 0x0c, 0x5c, 0x09, 0x3c, 0xdc, 0xb8, 0xe1, 0x46, 0xfc, 0x0a, 0x46,
	0x8e, 0xad, 0x79, 0x50, 0x48, 0x17, 0x91, 0x32, 0x77, 0x67, 0x2a, 0xa7, 0x60, 0x61, 0xe6, 0x4f,
	0xcb, 0x6a, 0x4f, 0x56, 0x2c, 0x77, 0xb8, 0x24, 0x08, 0x36, 0x28, 0x56, 0x60, 0x8b, 0xe4, 0x51,
	0x26, 0x0b, 0x2e, 0x34, 0x06, 0x22, 0x68, 0x60, 0xee, 0x68, 0xe1, 0x00, 0x84, 0x1a, 0xce, 0x50,
	0x31, 0x94, 0x2f, 0x75, 0xbb, 0x00, 0xd9, 0x63, 0x21, 0xe5, 0x7e, 0x47, 0xf5, 0x1a, 0x27, 0x7e,
	0x23, 0x24, 0x9d, 0x72, 0x6c, 0x63, 0x9f, 0x49, 0xeb, 0x22, 0xb8, 0xf7, 0x1f, 0x06, 0x00, 0x68,
	0x2e, 0x21, 0x9c, 0xe3, 0xd6, 0xa2, 0xd0, 0x7d, 0xa5, 0x3e, 0xbd, 0xeb, 0x15, 0xe2, 0x08, 0x15,
	0xf5, 0x42, 0xe7, 0xee, 0x31, 0x9d, 0xf3, 0x7a, 0x34, 0x8f, 0xd3, 0x56, 0x13, 0x67, 0xb3, 0x55,
	0x2f, 0xdb, 0xe1, 0x5d, 0xf4, 0x3e, 0xb1, 0x35, 0x63, 0x73, 0x31, 0xfc, 0xed, 0x56, 0x67, 0x80,
	0xc4, 0x62, 0x5d, 0x37, 0x4c, 0xf1, 0xa5, 0x64, 0xde, 0x56, 0xef, 0x92, 0x78, 0x01, 0x2b, 0x41,
	0xb6, 0x91, 0x75, 0xbc, 0x5d, 0x82, 0xe3, 0x56, 0xc0, 0x07, 0x2b, 0x24, 0xd4, 0x82, 0x1d, 0x75,
	0xdd, 0x77, 0x01, 0x04, 0xca, 0x60, 0x97, 0x26, 0x34, 0xf8, 0x6a, 0x8d, 0xc3, 0x99, 0x29, 0x61,
	0x7f, 0x5d, 0x9f, 0x11, 0x17, 0xee, 0x23, 0x5f, 0xf8, 0x6c, 0xd1, 0xac, 0x37, 0xbb, 0xe3, 0x1e,
	0xda, 0x55, 0xba, 0x7e, 0xb6, 0xbc, 0xf4, 0x03, 0x9c, 0x8a, 0x29, 0x4d, 0xa6, 0x55, 0xa1, 0x5d,
}

var testDecExpected = []byte(
	"1415926535897932384626433832795028841971693993751058209749445923078164062862089986280348253421170679" +
		"8214808651328230664709384460955058223172535940812848111745028410270193852110555964462294895493038196" +
		"4428810975665933446128475648233786783165271201909145648566923460348610454326648213393607260249141273" +
		"7245870066063155881748815209209628292540917153643678925903600113305305488204665213841469519415116094" +
		"3305727036575959195309218611738193261179310511854807446237996274956735188575272489122793818301194912" +
		"9833673362440656643086021394946395224737190702179860943702770539217176293176752384674818467669405132" +
		"0005681271452635608277857713427577896091736371787214684409012249534301465495853710507922796892589235" +
		"4201995611212902196086403441815981362977477130996051870721134999999837297804995105973173281609631859" +
		"5024459455346908302642522308253344685035261931188171010003137838752886587533208381420617177669147303" +
		"5982534904287554687311595628638823537875937519577818577805321712268066130019278766111959092164201989" +
		"3809525720106548586327886593615338182796823030195203530185296899577362259941389124972177528347913151" +
		"5574857242454150695950829533116861727855889075098381754637464939319255060400927701671139009848824012" +
		"8583616035637076601047101819429555961989467678374494482553797747268471040475346462080466842590694912" +
		"9331367702898915210475216205696602405803815019351125338243003558764024749647326391419927260426992279" +
		"6782354781636009341721641219924586315030286182974555706749838505494588586926995690927210797509302955" +
		"3211653449872027559602364806654991198818347977535663698074265425278625518184175746728909777727938000" +
		"8164706001614524919217321721477235014144197356854816136115735255213347574184946843852332390739414333" +
		"4547762416862518983569485562099219222184272550254256887671790494601653466804988627232791786085784383" +
		"8279679766814541009538837863609506800642251252051173929848960841284886269456042419652850222106611863" +
		"0674427862203919494504712371378696095636437191728746776465757396241389086583264599581339047802759009" +
		"9465764078951269468398352595709825822620522489407726719478268482601476990902640136394437455305068203" +
		"4962524517493996514314298091906592509372216964615157098583874105978859597729754989301617539284681382" +
		"6868386894277415599185592524595395943104997252468084598727364469584865383673622262609912460805124388" +
		"4390451244136549762780797715691435997700129616089441694868555848406353422072225828488648158456028506" +
		"01684273945226746767889525213852")

var testHexBytes = []byte{
	0xd3, 0x08, 0xa3, 0x85, 0x88, 0x6a, 0x3f, 0x24, 0x44, 0x73, 0x70, 0x03, 0x2e, 0x8a, 0x19, 0x13,
	0xd0, 0x31, 0x9f, 0x29, 0x22, 0x38, 0x09, 0xa4, 0x89, 0x6c, 0x4e, 0xec, 0x98, 0xfa, 0x2e, 0x08,
	0x77, 0x13, 0xd0, 0x38, 0xe6, 0x21, 0x28, 0x45, 0x6c, 0x0c, 0xe9, 0x34, 0xcf, 0x66, 0x54, 0xbe,
	0xdd, 0x50, 0x7c, 0xc9, 0xb7, 0x29, 0xac, 0xc0, 0x17, 0x09, 0x47, 0xb5, 0xb5, 0xd5, 0x84, 0x3f,
	0x1b, 0xfb, 0x79, 0x89, 0xd9, 0xd5, 0x16, 0x92, 0xac, 0xb5, 0xdf, 0x98, 0xa6, 0x0b, 0x31, 0xd1,
	0xb7, 0xdf, 0x1a, 0xd0, 0xdb, 0x72, 0xfd, 0x2f, 0x96, 0x7e, 0x26, 0x6a, 0xed, 0xaf, 0xe1, 0xb8,
	0x99, 0x7f, 0x2c, 0xf1, 0x45, 0x90, 0x7c, 0xba, 0xf7, 0x6c, 0x91, 0xb3, 0x47, 0x99, 0xa1, 0x24,
	0x16, 0xfc, 0x8e, 0x85, 0xe2, 0xf2, 0x01, 0x08, 0x69, 0x4e, 0x57, 0x71, 0xd8, 0x20, 0x69, 0x63,
	0x7e, 0x3d, 0x93, 0xf4, 0xa3, 0xfe, 0x58, 0xa4, 0x58, 0xb6, 0x8e, 0x72, 0x8f, 0x74, 0x95, 0x0d,
	0xee, 0x4a, 0x15, 0x82, 0x58, 0xcd, 0x8b, 0x71, 0xb5, 0x59, 0x5a, 0xc2, 0x1d, 0xa4, 0x54, 0x7b,
	0x13, 0x60, 0xf2, 0x2a, 0x39, 0xd5, 0x30, 0x9c, 0xf0, 0x85, 0x60, 0x28, 0x23, 0xb0, 0xd1, 0xc5,
	0xef, 0x38, 0xdb, 0xb8, 0x18, 0x79, 0x41, 0xca, 0x0e, 0x18, 0x3a, 0x60, 0xb0, 0xdc, 0x79, 0x8e,
	0x3e, 0x8a, 0x1e, 0xb0, 0x8b, 0x0e, 0x9e, 0x6c, 0x27, 0x4b, 0x31, 0xbd, 0xc1, 0x77, 0x15, 0xd7,
	0x60, 0x5c, 0x60, 0x55, 0xda, 0x2f, 0xaf, 0x78, 0x94, 0xab, 0x55, 0xaa, 0xf3, 0x25, 0x55, 0xe6,
	0x40, 0x14, 0xe8, 0x63, 0x62, 0x98, 0x48, 0x57, 0xb6, 0x10, 0xab, 0x2a, 0x6a, 0x39, 0xca, 0x55,
	0xce, 0xe8, 0x41, 0x11, 0x34, 0x5c, 0xcc, 0xb4, 0x93, 0xe9, 0x72, 0x7c, 0xaf, 0x86, 0x54, 0xa1,
	0x2a, 0xbc, 0x6f, 0x63, 0x11, 0x14, 0xee, 0xb3, 0xf6, 0x31, 0x18, 0x74, 0x5d, 0xc5, 0xa9, 0x2b,
	0x1e, 0x93, 0x87, 0x9b, 0x16, 0x3e, 0x5c, 0xce, 0x5c, 0xcf, 0x24, 0x6c, 0x33, 0xba, 0xd6, 0xaf,
	0x77, 0x86, 0x95, 0x28, 0x81, 0x53, 0x32, 0x7a, 0xaf, 0xb9, 0x4b, 0x6b, 0x98, 0x48, 0x8f, 0x3b,
	0x93, 0x21, 0x28, 0x66, 0x1b, 0xe8, 0xbf, 0xc4, 0x91, 0xa9, 0x21, 0xfb, 0xcc, 0x09, 0xd8, 0x61,
	0x32, 0x80, 0xec, 0x5d, 0x60, 0xac, 0x7c, 0x48, 0xb1, 0x75, 0x85, 0xe9, 0x5d, 0x5d, 0x84, 0xef,
	0x88, 0x1b, 0x65, 0xeb, 0x02, 0x23, 0x26, 0xdc, 0xc5, 0xac, 0x96, 0xd3, 0x81, 0x3e, 0x89, 0x23,
	0x39, 0x42, 0xf4, 0x83, 0xf3, 0x6f, 0x6d, 0x0f, 0x04, 0x20, 0x84, 0xa4, 0x82, 0x44, 0x0b, 0x2e,
	0x5e, 0x9b, 0x1f, 0x9e, 0x4a, 0xf0, 0xc8, 0x69, 0x9a, 0x6c, 0xe9, 0xf6, 0x42, 0x68, 0xc6, 0x21,
	0xf0, 0x88, 0xd3, 0xab, 0x61, 0x9c, 0x0c, 0x67, 0x68, 0x2f, 0x54, 0xd8, 0xd2, 0xa0, 0x51, 0x6a,
	0xa3, 0x33, 0x51, 0xab, 0x28, 0xa7, 0x0f, 0x96, 0xe4, 0x3b, 0x7a, 0x13, 0x6c, 0x0b, 0xef, 0x6e,
	0x98, 0x2a, 0xfb, 0x7e, 0x50, 0xf0, 0x3b, 0xba, 0x76, 0x01, 0xaf, 0x39, 0x1d, 0x65, 0xf1, 0xa1,
	0x88, 0x0e, 0x43, 0x82, 0x3e, 0x59, 0xca, 0x66, 0xb4, 0x9f, 0x6f, 0x45, 0x19, 0x86, 0xee, 0x8c,
	0xbe, 0x5e, 0x8b, 0x3b, 0xc3, 0xa5, 0x84, 0x7d, 0x73, 0x20, 0xc1, 0x85, 0xd8, 0x75, 0x6f, 0xe0,
	0xa6, 0x6a, 0xc1, 0x56, 0x9f, 0x44, 0x1a, 0x40, 0x06, 0x77, 0x3f, 0x36, 0x62, 0xaa, 0xd3, 0x4e,
	0x3d, 0x02, 0x9b, 0x42, 0x72, 0xdf, 0xfe, 0x1b, 0x48, 0x12, 0x0a, 0xd0, 0x24, 0xd7, 0xd0, 0x37,
	0x9b, 0xc0, 0xf1, 0x49, 0xd3, 0xea, 0x0f, 0xdb, 0x7b, 0x1b, 0x99, 0x80, 0xc9, 0x72, 0x53, 0x07,
	0xf7, 0xde, 0xe8, 0xf6, 0xd8, 0x79, 0xd4, 0x25, 0x3b, 0x4c, 0x79, 0xb6, 0x1a, 0x50, 0xfe, 0xe3,
	0xba, 0x06, 0xc0, 0x04, 0xbd, 0xe0, 0x6c, 0x97, 0xc4, 0x60, 0x9f, 0x40, 0xb6, 0x4f, 0xa9, 0xc1,
	0x63, 0x24, 0x6a, 0x19, 0xc2, 0x9e, 0x5c, 0x5e, 0xb5, 0x53, 0x6c, 0x3e, 0xaf, 0x6f, 0xfb, 0x68,
	0x6f, 0xec, 0x52, 0x3b, 0xeb, 0xb2, 0x39, 0x13, 0x2c, 0x95, 0x30, 0x9b, 0x1f, 0x51, 0xfc, 0x6d,
	0x09, 0xbd, 0x5e, 0xaf, 0x44, 0x45, 0x81, 0xcc, 0xfd, 0x4a, 0x33, 0xde, 0x04, 0xd0, 0xe3, 0xbe,
	0xb3, 0x4b, 0x2e, 0x19, 0x07, 0x28, 0x0f, 0x66, 0x0f, 0x74, 0xc8, 0x45, 0x57, 0xa8, 0xcb, 0xc0,
	0xdb, 0xfb, 0xd3, 0xb9, 0x39, 0x5f, 0x0b, 0xd2, 0x0a, 0x32, 0x60, 0x1a, 0xbd, 0xc0, 0x79, 0x55,
	0x79, 0x72, 0x2c, 0x40, 0xc6, 0x00, 0xa1, 0xd6, 0xcc, 0xa3, 0x1f, 0xfb, 0xfe, 0x25, 0x9f, 0x67,
	0xf8, 0x22, 0x32, 0xdb, 0xf8, 0xe9, 0xa5, 0x8e, 0x15, 0x6b, 0x61, 0xfd, 0xdf, 0x16, 0x75, 0x3c,
	0xab, 0x52, 0x05, 0xad, 0xc8, 0x1e, 0x50, 0x2f, 0x60, 0x87, 0x23, 0xfd, 0xfa, 0xb5, 0x3d, 0x32,
	0x82, 0xdf, 0x00, 0x3e, 0x48, 0x7b, 0x31, 0x53, 0xa0, 0x8c, 0x6f, 0xca, 0xbb, 0x57, 0x5c, 0x9e,
	0xdb, 0x69, 0x17, 0xdf, 0x2e, 0x56, 0x87, 0x1a, 0xc3, 0xff, 0x7e, 0x28, 0xf6, 0xa8, 0x42, 0xd5,
	0x73, 0x55, 0x4f, 0x8c, 0xc6, 0x32, 0x67, 0xac, 0xc8, 0x58, 0xca, 0xbb, 0xb0, 0x27, 0x5b, 0x69,
	0xa0, 0x11, 0xf0, 0xb8, 0x5d, 0xa3, 0xff, 0xe1, 0xb8, 0x83, 0x21, 0xfd, 0x98, 0x3d, 0xfa, 0x10,
	0x5b, 0xd3, 0xd1, 0x2d, 0x6c, 0xb5, 0xfc, 0x4a, 0x65, 0x45, 0xf8, 0xb6, 0x79, 0xe4, 0x53, 0x9a,
	0x90, 0x97, 0xfb, 0x4b, 0xbc, 0x49, 0x8e, 0xd2, 0x33, 0x7e, 0xcb, 0xa4, 0xda, 0xf2, 0xdd, 0xe1,
	0xe8, 0xc6, 0xe4, 0xce, 0x41, 0x13, 0xfb, 0x62, 0x01, 0x4c, 0x77, 0x36, 0xda, 0xca, 0x20, 0xef,
	0xb4, 0x1f, 0xf1, 0x2b, 0xfe, 0x9e, 0x7e, 0xd0, 0x98, 0x91, 0x90, 0xae, 0x4d, 0xda, 0xdb, 0x95,
	0xa0, 0xd5, 0x93, 0x6b, 0x71, 0x8e, 0xad, 0xea, 0xe0, 0x25, 0xc7, 0xaf, 0xd0, 0xd1, 0x8e, 0xd0,
	0xb7, 0x94, 0x75, 0x8e, 0x2f, 0x5b, 0x3c, 0x8e, 0x64, 0x2b, 0x12, 0xf2, 0xfb, 0xe2, 0xf6, 0x8f,
	0x1c, 0xf0, 0x0d, 0x90, 0x12, 0xb8, 0x88, 0x88, 0x1c, 0xc3, 0x8f, 0x68, 0xa0, 0x5e, 0xad, 0x4f,
	0xad, 0xc1, 0xa8, 0xb3, 0x91, 0xf1, 0xcf, 0xd1, 0x77, 0x17, 0x0e, 0xbe, 0x18, 0x22, 0x2f, 0x2f,
	0xa1, 0x1f, 0x02, 0x8b, 0xfe, 0x2d, 0x75, 0xea, 0xe8, 0x74, 0x6f, 0xb5, 0x0f, 0xcc, 0xa0, 0xe5,
	0x99, 0xe2, 0x89, 0xce, 0xd6, 0xf3, 0xac, 0x18, 0xb7, 0xe0, 0x13, 0xfd, 0xe0, 0x4f, 0xa8, 0xb4,
	0xd9, 0xa8, 0xad, 0xd2, 0x81, 0x3b, 0xc4, 0x7c, 0x05, 0x77, 0x95, 0x80, 0x66, 0xa2, 0x5f, 0x16,
	0x77, 0x14, 0x1a, 0x21, 0x14, 0x73, 0xcc, 0x93, 0x86, 0xfa, 0xb5, 0x77, 0x65, 0x20, 0xad, 0xe6,
	0xcf, 0x35, 0x9d, 0xfb, 0xf5, 0x42, 0x54, 0xc7, 0xa0, 0x89, 0x3e, 0x7b, 0x0c, 0xaf, 0xcd, 0xeb,
	0x49, 0x7e, 0x1e, 0xae, 0xd3, 0x1b, 0x41, 0xd6, 0x5e, 0xb3, 0x71, 0x20, 0x2d, 0x0e, 0x25, 0x00,
	0xaf, 0xe0, 0xb8, 0x57, 0xbb, 0x00, 0x68, 0x22, 0x1e, 0xb9, 0x09, 0xf0, 0x9b, 0x36, 0x64, 0x24,
	0xaa, 0xa6, 0xdf, 0x59, 0x1d, 0x91, 0x63, 0x55, 0x7f, 0x53, 0x5a, 0xd9, 0x89, 0x43, 0xc1, 0x78,
	0xc5, 0xb9, 0xe5, 0x02, 0xa2, 0x5b, 0x7d, 0x20, 0xa9, 0xcf, 0x95, 0x62, 0x76, 0x03, 0x26, 0x83,
	0x41, 0x4a, 0x73, 0x4e, 0x68, 0x19, 0xc8, 0x11, 0x4a, 0xa9, 0x14, 0x7b, 0xca, 0x2d, 0x47, 0xb3,
}

var testHexExpected = []byte(
	"243f6a8885a308d313198a2e03707344a4093822299f31d0082efa98ec4e6c89452821e638d01377be5466cf34e90c6cc0ac" +
		"29b7c97c50dd3f84d5b5b54709179216d5d98979fb1bd1310ba698dfb5ac2ffd72dbd01adfb7b8e1afed6a267e96ba7c9045" +
		"f12c7f9924a19947b3916cf70801f2e2858efc16636920d871574e69a458fea3f4933d7e0d95748f728eb658718bcd588215" +
		"4aee7b54a41dc25a59b59c30d5392af26013c5d1b023286085f0ca417918b8db38ef8e79dcb0603a180e6c9e0e8bb01e8a3e" +
		"d71577c1bd314b2778af2fda55605c60e65525f3aa55ab945748986263e8144055ca396a2aab10b6b4cc5c341141e8cea154" +
		"86af7c72e993b3ee1411636fbc2a2ba9c55d741831f6ce5c3e169b87931eafd6ba336c24cf5c7a325381289586773b8f4898" +
		"6b4bb9afc4bfe81b6628219361d809ccfb21a991487cac605dec8032ef845d5de98575b1dc262302eb651b8823893e81d396" +
		"acc50f6d6ff383f442392e0b4482a484200469c8f04a9e1f9b5e21c66842f6e96c9a670c9c61abd388f06a51a0d2d8542f68" +
		"960fa728ab5133a36eef0b6c137a3be4ba3bf0507efb2a98a1f1651d39af017666ca593e82430e888cee8619456f9fb47d84" +
		"a5c33b8b5ebee06f75d885c12073401a449f56c16aa64ed3aa62363f77061bfedf72429b023d37d0d724d00a1248db0fead3" +
		"49f1c09b075372c980991b7b25d479d8f6e8def7e3fe501ab6794c3b976ce0bd04c006bac1a94fb6409f60c45e5c9ec2196a" +
		"246368fb6faf3e6c53b51339b2eb3b52ec6f6dfc511f9b30952ccc814544af5ebd09bee3d004de334afd660f2807192e4bb3" +
		"c0cba85745c8740fd20b5f39b9d3fbdb5579c0bd1a60320ad6a100c6402c7279679f25fefb1fa3cc8ea5e9f8db3222f83c75" +
		"16dffd616b152f501ec8ad0552ab323db5fafd23876053317b483e00df829e5c57bbca6f8ca01a87562edf1769dbd542a8f6" +
		"287effc3ac6732c68c4f5573695b27b0bbca58c8e1ffa35db8f011a010fa3d98fd2183b84afcb56c2dd1d35b9a53e479b6f8" +
		"4565d28e49bc4bfb9790e1ddf2daa4cb7e3362fb1341cee4c6e8ef20cada36774c01d07e9efe2bf11fb495dbda4dae909198" +
		"eaad8e716b93d5a0d08ed1d0afc725e08e3c5b2f8e7594b78ff6e2fbf2122b648888b812900df01c4fad5ea0688fc31cd1cf" +
		"f191b3a8c1ad2f2f2218be0e1777ea752dfe8b021fa1e5a0cc0fb56f74e818acf3d6ce89e299b4a84fe0fd13e0b77cc43b81" +
		"d2ada8d9165fa2668095770593cc7314211a1477e6ad206577b5fa86c75442f5fb9d35cfebcdaf0c7b3e89a0d6411bd3ae1e" +
		"7e4900250e2d2071b35e226800bb57b8e0af2464369bf009b91e5563911d59dfa6aa78c14389d95a537f207d5ba202e5b9c5" +
		"832603766295cfa911c819684e734a41b3472dca7b14a94a")

var testDecMultipleBlocks = []byte{
	0x60, 0xe2, 0x3e, 0xb8, 0xae, 0x61, 0xa6, 0x13,
	0x00, 0x0f, 0x58, 0xf3, 0x84, 0x66, 0xef, 0x56,
	// Block Boundary
	0x17, 0x3f, 0x65, 0x1a, 0x21, 0x09, 0xca, 0x45,
	0x00, 0x60, 0x5b, 0x4a, 0x96, 0x06, 0x14, 0x08,
	// Block Boundary
	0x09, 0xfb, 0xd6, 0x59, 0x35, 0x00, 0x33, 0x52,
	0x00, 0xe9, 0xe5, 0x0f, 0x83, 0xb7, 0xdf, 0x88,
	// Block Boundary
	0x00, 0xe6, 0xc6, 0x3d, 0x9b, 0x70, 0x7a, 0x2f,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // dummy data (reader should ignore this).
	// Block Boundary
}
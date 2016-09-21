package protocol

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"github.com/google/flatbuffers/go"
	"io"

	"idls/tilerequests"
)

// CaptureImageRequest defines the request fields
type CaptureImageRequest struct {
	AcqID    uint64
	Camera   int32
	Frame    int32
	Col      int32
	Row      int32
	Image    []byte
	Checksum []byte
}

// CaptureImageResponse defines the response fields
type CaptureImageResponse struct {
	AcqID              uint64
	TileID             int64
	Status             int16
	SystemStatus       int16
	TileQueueStatus    int16
	ContentQueueStatus int16
}

// ReadUint32 - reads a uint32 value
func ReadUint32(reader io.Reader) (uint32, error) {
	var valuebuffer [4]byte
	n, err := reader.Read(valuebuffer[0:])
	if err != nil {
		return 0, err
	}
	if n < 4 {
		return 0, fmt.Errorf("Expected to read 4 bytes")
	}
	return flatbuffers.GetUint32(valuebuffer[0:]), nil
}

// WriteUint32 - writes a uint32 value
func WriteUint32(writer io.Writer, v uint32) error {
	var valuebuffer [4]byte
	flatbuffers.WriteUint32(valuebuffer[0:], v)

	n, err := writer.Write(valuebuffer[0:])
	if err != nil {
		return err
	}
	if n < 4 {
		return fmt.Errorf("Expected to write 4 bytes for %d", v)
	}
	return nil
}

// MarshalTileRequest - serializes the request
func MarshalTileRequest(req *CaptureImageRequest) []byte {
	b := flatbuffers.NewBuilder(len(req.Image) + 1024)

	image := b.CreateByteVector(req.Image)
	checksum := b.CreateByteVector(req.Checksum)

	tilerequests.ImageTileRequestStart(b)
	tilerequests.ImageTileRequestAddAcqId(b, req.AcqID)
	tilerequests.ImageTileRequestAddCamera(b, req.Camera)
	tilerequests.ImageTileRequestAddFrame(b, req.Frame)
	tilerequests.ImageTileRequestAddCol(b, req.Col)
	tilerequests.ImageTileRequestAddRow(b, req.Row)
	tilerequests.ImageTileRequestAddImage(b, image)
	tilerequests.ImageTileRequestAddChecksum(b, checksum)
	done := tilerequests.ImageTileRequestEnd(b)
	b.Finish(done)

	return b.Bytes[b.Head():]
}

// UnmarshalTileRequest - unserialize the request
func UnmarshalTileRequest(requestBuffer []byte) (acqID uint64, camera, frame, col, row, imageSize int, image, checksum []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	imageTileRequest := tilerequests.GetRootAsImageTileRequest(requestBuffer, 0)
	acqID = imageTileRequest.AcqId()
	camera = int(imageTileRequest.Camera())
	frame = int(imageTileRequest.Frame())
	col = int(imageTileRequest.Col())
	row = int(imageTileRequest.Row())
	imageSize = imageTileRequest.ImageLength()
	image = imageTileRequest.ImageBytes()
	checksum = imageTileRequest.ChecksumBytes()

	computedChecksum := md5.Sum(image)
	if !bytes.Equal(computedChecksum[0:], checksum) {
		err = fmt.Errorf("The received checksum %x and the calculated checksum %x do not match", checksum, computedChecksum)
	}
	return acqID, camera, frame, col, row, imageSize, image, checksum, err
}

// MarshalTileResponse - serialize the response
func MarshalTileResponse(resp *CaptureImageResponse) []byte {
	b := flatbuffers.NewBuilder(1024)

	tilerequests.ImageTileResponseStart(b)
	tilerequests.ImageTileResponseAddAcqId(b, resp.AcqID)
	tilerequests.ImageTileResponseAddTileId(b, resp.TileID)
	tilerequests.ImageTileResponseAddStatus(b, resp.Status)
	tilerequests.ImageTileResponseAddSystemStatus(b, resp.SystemStatus)
	tilerequests.ImageTileResponseAddTileQueueStatus(b, resp.TileQueueStatus)
	tilerequests.ImageTileResponseAddContentQueueStatus(b, resp.ContentQueueStatus)
	done := tilerequests.ImageTileResponseEnd(b)
	b.Finish(done)

	return b.Bytes[b.Head():]
}

// UnmarshalTileResponse - unserialize the response
func UnmarshalTileResponse(responsebuffer []byte) *CaptureImageResponse {
	imageTileResponse := tilerequests.GetRootAsImageTileResponse(responsebuffer, 0)
	tileResponse := &CaptureImageResponse{}
	tileResponse.AcqID = imageTileResponse.AcqId()
	tileResponse.TileID = imageTileResponse.TileId()
	tileResponse.Status = imageTileResponse.Status()
	tileResponse.SystemStatus = int16(imageTileResponse.SystemStatus())
	tileResponse.ContentQueueStatus = int16(imageTileResponse.ContentQueueStatus())
	tileResponse.TileQueueStatus = int16(imageTileResponse.TileQueueStatus())
	return tileResponse
}

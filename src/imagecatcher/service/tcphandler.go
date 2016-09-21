package service

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"time"

	"imagecatcher/logger"
	"imagecatcher/models"
	"imagecatcher/protocol"
	"imagecatcher/utils"
)

// tcpServerHandler handles request coming over a TCP/IP socket
type tcpServerHandler struct {
	l   net.Listener
	trh *TileRequestHandler
}

// NewTCPServerHandler creates an instance of a request Handler
func NewTCPServerHandler(l net.Listener, trh *TileRequestHandler) ServerHandler {
	h := &tcpServerHandler{l, trh}
	return h
}

// Serve implementation of a ServerHandler method
func (h *tcpServerHandler) Serve() (err error) {
	for {
		var e error
		var conn net.Conn
		if conn, e = h.l.Accept(); e != nil {
			logger.Errorf("tcp: Accept error: %v", e)
			continue
		}
		go h.handle(conn)
	}
}

func (h *tcpServerHandler) handle(conn net.Conn) {
	for {
		var err error
		if err = h.handleCaptureImageRequest(conn, conn); err == io.EOF {
			if closeErr := conn.Close(); closeErr != nil {
				logger.Errorf("Unexpected error while closing the connection: %v", closeErr)
			}
			break
		}
		if err != nil {
			logger.Errorf("Unexpected error: %v", err)
		}
	}
}

func (h *tcpServerHandler) handleCaptureImageRequest(reader io.Reader, writer io.Writer) (err error) {
	tileJob := &tileProcessingJob{}
	tileJob.execCtx.StartTime = time.Now()
	defer tileJob.clear()

	if _, err = h.readTileRequest(tileJob, reader); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			logger.Errorf("Read request error encountered: %v", err)
			h.writeTileResponse(writer, tileJob, 0, err)
		}
		return err
	}
	logger.Debugf("End request parsing for %d:%s (%d) %v",
		tileJob.acqID, tileJob.tileParams.Name, tileJob.tileParams.ContentLen, time.Since(tileJob.execCtx.StartTime))

	var tileID int64
	var tileInfo *models.TemImage
	tileInfo, err = h.trh.startTileProcessingJob(tileJob)
	if tileInfo != nil {
		tileID = tileInfo.ImageID
	}
	h.writeTileResponse(writer, tileJob, tileID, err)
	return
}

func (h *tcpServerHandler) readTileRequest(tileJob *tileProcessingJob, reader io.Reader) (n int64, err error) {
	var requestBufferLength uint32
	if requestBufferLength, err = protocol.ReadUint32(reader); err != nil {
		if err == io.EOF {
			logger.Debugf("Connection closed")
			return
		}
		logger.Errorf("Error reading the total buffer length from the connection %v", err)
		return
	}
	// allocate a little bit more than the request buffer length to avoid reallocations
	requestBufferPtr, requestBufferBytes := utils.Alloc(requestBufferLength + bytes.MinRead + 1)
	requestBuffer := bytes.NewBuffer(requestBufferBytes[0:0])
	tileJob.memPtr = requestBufferPtr

	n, err = requestBuffer.ReadFrom(io.LimitReader(reader, int64(requestBufferLength)))

	if err != nil {
		if err == io.EOF {
			logger.Errorf("Unexpected EOF")
			return
		}
		return
	}
	if uint32(n) < requestBufferLength {
		return n, fmt.Errorf("Expected to read %d bytes but instead it only read %d. This may result into a 'Unmarshalling error'!", requestBufferLength, n)
	}
	logger.Debugf("End reading buffer (%d) %v", requestBufferLength, time.Since(tileJob.execCtx.StartTime))

	tileJob.acqID,
		tileJob.tileParams.Camera,
		tileJob.tileParams.Frame,
		tileJob.tileParams.Col,
		tileJob.tileParams.Row,
		tileJob.tileParams.ContentLen,
		tileJob.tileParams.Content,
		tileJob.tileParams.Checksum,
		err = protocol.UnmarshalTileRequest(requestBufferBytes)

	tileJob.tileParams.UpdateTileName()
	return
}

func (h *tcpServerHandler) writeTileResponse(writer io.Writer, tileJob *tileProcessingJob, tileID int64, perr error) (werr error) {
	var status int16
	var systemStatus QueueStatus
	systemStatus, tileQueueStatus, contentQueueStatus := h.trh.getProcessingQueuesStatus()
	if perr != nil {
		status = 1
		systemStatus = Red // automatically make system RED in case of an error
	} else {
		status = 0
	}
	responseBuf := protocol.MarshalTileResponse(&protocol.CaptureImageResponse{
		AcqID:              tileJob.acqID,
		TileID:             tileID,
		Status:             status,
		SystemStatus:       int16(systemStatus),
		TileQueueStatus:    int16(tileQueueStatus),
		ContentQueueStatus: int16(contentQueueStatus),
	})

	if werr = protocol.WriteUint32(writer, uint32(len(responseBuf))); werr != nil {
		logger.Errorf("Error writing the total response buffer length for %d:%s (%d bytes): %v",
			tileJob.acqID, tileJob.tileParams.Name, tileJob.tileParams.ContentLen, werr)
	}
	if _, werr = writer.Write(responseBuf); werr != nil {
		logger.Errorf("Error writing the response buffer for %d:%s (%d bytes): %v",
			tileJob.acqID, tileJob.tileParams.Name, tileJob.tileParams.ContentLen, werr)
	}
	logger.Infof("End sending the response for %d:%s (%d) %v",
		tileJob.acqID, tileJob.tileParams.Name, tileJob.tileParams.ContentLen, time.Since(tileJob.execCtx.StartTime))
	return werr
}

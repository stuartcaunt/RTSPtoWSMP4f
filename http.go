package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/mp4f"

	"github.com/gin-gonic/gin"
)

type StreamConnectionRequest struct {
	URL string
}

type SteamConnection struct {
	suuid          string
	connectionURLs []string
}

var streamConnections map[string]SteamConnection

func serveHTTP() {
	streamConnections = make(map[string]SteamConnection)

	router := gin.Default()
	gin.SetMode(gin.DebugMode)

	router.GET("api/streams", func(c *gin.Context) {
		streams := Config.getStreamNames()
		sort.Strings(streams)
		c.JSON(http.StatusOK, streams)
	})

	router.POST("api/streams/:suuid/connect", func(c *gin.Context) {
		var suuid = c.Param("suuid")
		var streamConnectionRequest StreamConnectionRequest

		if !Config.streamExists(suuid) {
			log.Println(fmt.Sprintf("Stream %s does not exist", suuid))
			c.String(http.StatusNotFound, "Not found")
			return
		}

		if err := c.BindJSON(&streamConnectionRequest); err != nil {
			log.Println(fmt.Sprintf("Error connecting to stream %s: %v", suuid, err))
			c.String(http.StatusInternalServerError, "Internal error")
			return
		}

		connectStream(suuid, streamConnectionRequest.URL)
		c.JSON(http.StatusOK, streamConnectionRequest)
	})

	router.POST("api/streams/:suuid/disconnect", func(c *gin.Context) {
		var suuid = c.Param("suuid")
		var streamConnectionRequest StreamConnectionRequest

		if !Config.streamExists(suuid) {
			log.Println(fmt.Sprintf("Stream %s does not exist", suuid))
			c.String(http.StatusNotFound, "Not found")
			return
		}

		if err := c.BindJSON(&streamConnectionRequest); err != nil {
			log.Println(fmt.Sprintf("Error disconnecting from stream %s: %v", suuid, err))
			c.String(http.StatusInternalServerError, "Fail")
			return
		}

		disconnectStream(suuid, streamConnectionRequest.URL)
		c.JSON(http.StatusOK, streamConnectionRequest)
	})

	// Start the HTTP server
	err := router.Run(Config.Server.HTTPPort)
	if err != nil {
		log.Fatalln(err)
	}
}

func connectStream(suuid string, connectionURL string) {
	// Get the current connections for the suuid
	currentConnections, exists := streamConnections[suuid]
	if !exists {
		// Add a new StreamConnection if it is the first one for this stream
		streamConnections[suuid] = SteamConnection{suuid, []string{connectionURL}}

		log.Println(fmt.Sprintf("Conection URL %s added to stream %s. Stream will be started.", connectionURL, suuid))

		go streamRelay(suuid, connectionURL)

	} else {
		// Determine if the connection URL has already been added
		existingConnectionURLIndex := sort.SearchStrings(currentConnections.connectionURLs, connectionURL)
		if existingConnectionURLIndex < len(currentConnections.connectionURLs) {
			// ConnectionURL is alreay configured
			log.Println(fmt.Sprintf("Conection URL %s is already connected to stream %s", connectionURL, suuid))

		} else {
			// new ConnectionURL
			log.Println(fmt.Sprintf("Conection URL %s added to stream %s", connectionURL, suuid))
			currentConnections.connectionURLs = append(currentConnections.connectionURLs, connectionURL)

			go streamRelay(suuid, connectionURL)
		}
	}
}

func disconnectStream(suuid string, connectionURL string) {
	// Get the current connections for the suuid
	currentConnections, exists := streamConnections[suuid]
	if exists {
		// Determine if the connection URL has already been added
		existingConnectionURLIndex := sort.SearchStrings(currentConnections.connectionURLs, connectionURL)
		if existingConnectionURLIndex < len(currentConnections.connectionURLs) {
			// ConnectionURL is configured, let's remove it
			currentConnections.connectionURLs = append(currentConnections.connectionURLs[:existingConnectionURLIndex], currentConnections.connectionURLs[existingConnectionURLIndex+1:]...)

			log.Println(fmt.Sprintf("Conection URL %s disconnected from stream %s", connectionURL, suuid))

			// Check for emty connectionURLs
			if len(currentConnections.connectionURLs) == 0 {
				log.Println(fmt.Sprintf("No active connections to stream %s. Stream will be stopped.", suuid))

				delete(streamConnections, suuid)
			}
		}
	}
}

func connectionExists(suuid string, connectionURL string) bool {
	currentConnections, exists := streamConnections[suuid]
	if exists {
		existingConnectionURLIndex := sort.SearchStrings(currentConnections.connectionURLs, connectionURL)
		if existingConnectionURLIndex < len(currentConnections.connectionURLs) {
			return true
		}
	}
	return false
}

func streamRelay(suuid string, connectionURL string) {
	if !Config.streamExists(suuid) {
		log.Println("Stream Not Found")
		return
	}

	// Disconnect the client when method out of scope
	defer disconnectStream(suuid, connectionURL)

	// Start RTSP Client if it isn't running already
	Config.RunIFNotRun(suuid)

	// Add a new client UUID for the stream. Return the client UUID and the data channel
	cuuid, ch := Config.addClient(suuid)

	fullUrl := connectionURL + "/" + suuid

	// Remove the client when this method goes out of scope
	defer Config.deleteClient(suuid, cuuid)

	// Get the codecs of the stream
	codecs := Config.coGe(suuid)
	if codecs == nil {
		log.Println("Codecs Error")
		return
	}
	for i, codec := range codecs {
		if codec.Type().IsAudio() && codec.Type() != av.AAC {
			log.Println("Track", i, "Audio Codec Work Only AAC")
		}
	}

	// Create a new fMP4 muxer
	muxer := mp4f.NewMuxer(nil)
	err := muxer.WriteHeader(codecs)
	if err != nil {
		log.Println("muxer.WriteHeader", err)
		return
	}

	// Get initial data
	meta, init := muxer.GetInit(codecs)
	log.Println(fmt.Sprintf("[%s] Sending meta (%s) and init (%d bytes) to client", connectionURL, meta, len(init)))

	// Send header
	err = postData(fullUrl, append([]byte{9}, meta...))
	if err != nil {
		return
	}

	// Send init
	err = postData(fullUrl, init)
	if err != nil {
		return
	}

	var start bool

	// Check for no video every 10s
	noVideo := time.NewTimer(10 * time.Second)
	var timeLine = make(map[int8]time.Duration)
	for {
		select {
		// No video after 10s
		case <-noVideo.C:
			log.Println("noVideo")
			return

		// Data from the RTSP client
		case pck := <-ch:
			// Check if connection is still valid
			if !connectionExists(suuid, connectionURL) {
				log.Println(fmt.Sprintf("[%s] client has disconnected. Stopping relay.", connectionURL))
				return
			}

			// Check for keyframe
			if pck.IsKeyFrame {
				noVideo.Reset(10 * time.Second)
				start = true
			}

			// Wait for a key frame
			if !start {
				continue
			}

			timeLine[pck.Idx] += pck.Duration
			pck.Time = timeLine[pck.Idx]

			// Transform the packet to fMP4
			ready, buf, _ := muxer.WritePacket(pck, false)
			if ready {
				log.Println(fmt.Sprintf("[%s] Sending data buffer (%d bytes)", fullUrl, len(buf)))

				err = postData(fullUrl, buf)
				if err != nil {
					return
				}
			}
		}
	}
}

func postData(url string, data []byte) error {
	_, err := http.Post(url, "application/octet-stream", bytes.NewReader(data))
	if err != nil {
		log.Println(fmt.Sprintf("[%s] Failed to send data", url))
		return err
	}
	return nil
}

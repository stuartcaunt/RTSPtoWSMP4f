# RTSP to fMP4 over HTTP

> This project has been forked from https://github.com/deepch/RTSPtoWSMP4f 

This project converts the original code from deepch to provide a webservice for streaming fMP4 over HTTP rather than integrating the streams into a client via websockets.

The API allows a client to 
 - List available streams
 - Connect to a stream
 - Disconnect from a stream

When a client requests a stream connection, it passes a callback URL to which the server POSTs fMP4 data. The first connection to a stream launches the RTSP decoder and when no clients are connected the decoder stops. Each client has a MUXer to handle the repackaging of the video stream to fMP4.

 ## Building and running

1. Download source
   ```bash 
   git clone https://github.com/stuartcaunt/RTSPtoWSMP4f  
   ```
2. CD to Directory

   ```bash
    cd RTSPtoWSMP4f/
   ```
3. Run the server
   ```bash
    GO111MODULE=on go run *.go
   ```

## Configuration

### Edit file config.json

format:

```bash
{
  "server": {
    "http_port": ":8083"
  },
  "streams": {
   "H264_AAC": {
      "url": "rtsp://wowzaec2demo.streamlock.net/vod/mp4:BigBuckBunny_115k.mov"
    }
  }
}
```

## Testing

### CURL requests examples

The following assume the server is running on localhost on port 8083.

1. Getting a list of streams
> Endpoint: `GET /api/streams`
```bash
curl -X GET  http://localhost:8083/api/streams
```

2. Connecting to a stream
> Endpoint: `POST /api/streams/{streamId}/connect`
```bash
curl -X POST  http://localhost:8083/api/streams/Stream1/connect - d '{"url": "http://localhost:4000/api/streams"}'
```

> Note: The server will POST video stream data to `http://localhost:8000/api/streams/Stream1`

3. Disconnecting from a stream
> Endpoint: `POST /api/streams/{streamId}/disconnect`
```bash
curl -X POST  http://localhost:8083/api/streams/Stream1/disconnect - d '{"url": "http://localhost:4000/api/streams"}'
```

### Demo client

The project offers a simple node test client to test the reception of POSTed data

1. CD to Directory

   ```bash
    cd demo-client
   ```
2. Install dependencies
   ```bash
    npm install
   ```
3. Run the client application
   ```bash
    npm start
   ```

By default the client runs on port 4000.

## Limitations

Video Codecs Supported: H264 all profiles, H265 work only safari and (IE hw video card)

Audio Codecs Supported: AAC

## Test

CPU usage 0.2% one core cpu intel core i7 / stream

# Flowdownloader

FlowPlayer allows you to load video on the Amazon servers and uses the HLS (Http Live Streaming) protocol to get and play the streaming.
On the HTML page that embeds the player you'll find a link to the main index file (.m3u8). On this file you could find different type of streaming resolution. These are link to the secondary index.
The secondary index contains all the segments of the video.
So the trick is download all the video segments and merge them in a single mp4 file, but this player can also use the encryption. I saw it uses a AES-128 CBC encryption algorithm.
It is possible found the IV directly on the secondary index, but for the encryption key you'll find only a link to a page in the main site that embeds the player.

Sometimes to get the key is not enough call the url on the secondary index, because the page could be behind an authentication system or it use the session to call other page before this one to "unblock" the key. So the easy way to get the key is open the video on the browser, play it, open the Inspection tool and see all the XHR calls until to the page that contains the key. Grab it and use it with the -k parameter of Flowdownloader.

## Installation
To install the tool you have to clone this repo and simply use the following command:

    $ make

or to cross compile from Linux to Windows

    $ make windows

## Usage

    Usage of flowdownloader:
      -k string
            AES key
      -o string
            Output File (default "video.mp4")
      -u string
            Url m3u8

## Example

A simple example page that I used to make some tests is https://flowplayer.blacktrash.org/hls-crypt/ 

    $ ./flowdownloader -u "https://d12zt1n3pd4xhr.cloudfront.net/fp/drive-crypt5.m3u8" -k "gxsMQ8Iy1mzyJY9Addu1oQ=="  
    Choose resolution:
     0) 1280x720
     1) 1024x576
     2) 960x540
     3) 640x360
     4) 480x270
     5) 384x216
     6) 384x216
    > 6
    [@] Downloading 23/23
    [@] Decrypted 23/23
    [@] Completed


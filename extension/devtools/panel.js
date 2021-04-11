var keyURL = "";

class UI {
    key = "";
    primaryURL = "";
    secondaryURL = "";
    IV = "";

    constructor() {
        $('#spinner').css('visibility', 'visible');
        $('#content').hide();
    }

    setPrimaryUrl(url) {
        this.primaryURL = url;
        this.renderCommand();
    }

    setSecondaryUrl(url) {
        this.secondaryURL = url;
        this.renderCommand();
    }

    setIV(iv) {
        this.IV = iv;
        this.renderCommand();
    }

    setKey(key) {
        this.key = key;
        this.renderCommand();
    }

    renderCommand() {
        var command = "flowdownloader";
        if(this.key != "") {
            command += " -k \"" + this.key + "\""
            $('#key_txt').val(this.key);
        }

        if(this.IV != "") {
            $('#iv_txt').val(this.IV);
        }

        if(this.secondaryURL != "") {
            $('#secondary_txt').val(this.secondaryURL);
        }

        if(this.primaryURL != "") {
            command += " -u " + this.primaryURL;
            $('#primary_txt').val(this.primaryURL);
        } else {
            command += " -s -u " + this.secondaryURL;
        }

        $('#command').val(command);
        $('#spinner').css('visibility', 'hidden');
        $('#content').show();
    }
}

var ui = new UI();

chrome.devtools.network.onRequestFinished.addListener(
    function(data) {
        var u = data.request.url.split('?')[0];

        if(u.endsWith(".m3u8")) {
            console.log("request received: ", data.request.url);
            var currentURL = data.request.url;
            
            data.getContent(
                function(content, encoding){
                    if(encoding == "base64") {
                        content = atob(content);
                    }

                    //#EXT-X-KEY:METHOD=AES-128,URI="https://site.com/key_handle.php?key=video_1"
                    var lines = content.split('\n');
                    for(var i = 0;i < lines.length;i++){
                        console.log("analysis:", lines[i]);
                        if( lines[i].startsWith("#EXT-X-VERSION:")) {
                            // probably it is the secondary
                            ui.setSecondaryUrl(currentURL);
                        }
                        if(lines[i].startsWith("#EXT-X-KEY:METHOD=")) {
                            // Key URL extraction
                            var matches = lines[i].match(/URI=\"([^"]+)\"/);
                            if(matches && matches.length == 2) {
                                ui.setSecondaryUrl(currentURL);
                                console.log("key url found:", matches[1]);
                                keyURL = matches[1];

                                // IV extraction
                                matches = lines[i].match(/IV=([^,\s]+)/);
                                if(matches && matches.length == 2) {
                                    console.log("IV found:", matches[1]);
                                    ui.setIV(matches[1]);
                                }
                            }
                            break;
                        } else if(lines[i].startsWith("#EXT-X-STREAM-INF:")) {
                            // is the primary URL
                            ui.setPrimaryUrl(currentURL);
                            break;
                        }
                    }
                }
            );
        } else if(data.request.url == keyURL) {
            data.getContent(
                function(content, encoding){
                    if(encoding != "base64") {
                        content = btoa(content);
                    }
                    // key found
                    ui.setKey(content);
                    console.log("KEY: ",content);
                    keyURL       = "";
                }
            );
        }
    }
);

function copyToClipboard(text) {
    const input = document.createElement('input');
    input.style.position = 'fixed';
    input.style.opacity = 0;
    input.value = text;
    document.body.appendChild(input);
    input.select();
    document.execCommand('Copy');
    document.body.removeChild(input);
};

document.addEventListener('DOMContentLoaded', function() {
    document.getElementById('copyBtn').addEventListener('click', function() {
        var txt = document.getElementById('command').value;
        console.log("copying to clipboard", txt);
        copyToClipboard(txt);
    });
});
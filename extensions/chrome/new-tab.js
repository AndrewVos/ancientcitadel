  var xhr = new XMLHttpRequest();
  xhr.open("GET", 'http://ancientcitadel.com/api/random/sfw', true);
  xhr.onreadystatechange = function() {
    if (xhr.readyState == 4) {
      var gifElement = document.getElementById("gif");
      var randomGif = JSON.parse(xhr.responseText);

      var topArea = document.createElement("div");
      topArea.setAttribute("class", "top-area");

      var title = document.createElement("p");
      title.appendChild(document.createTextNode(randomGif.Title));
      topArea.appendChild(title);

      var link = document.createElement("a");
      link.setAttribute("href", "http://ancientcitadel.com/gif/"+randomGif.ID);
      link.appendChild(document.createTextNode("link"));
      topArea.appendChild(link);

      topArea.appendChild(document.createTextNode(" / "));

      var comments = document.createElement("a");
      comments.setAttribute("href", randomGif.SourceURL);
      comments.appendChild(document.createTextNode("comments"));
      topArea.appendChild(comments);

      gifElement.appendChild(topArea);

      var video = document.createElement("video"); 
      video.setAttribute("loop", true);
      video.setAttribute("autoplay", true);
      var webmSource = document.createElement("source");
      webmSource.setAttribute("src", randomGif.WebMURL);
      webmSource.setAttribute("type", "video/webm");
      video.appendChild(webmSource);
      gifElement.appendChild(video);


        // {{.Title}}
        // <br>
        // <a id="link" href="{{.URL}}">link</a> / <a id="source-url" href="{{.SourceURL}}">comments</a>
    }
  }
  xhr.send();

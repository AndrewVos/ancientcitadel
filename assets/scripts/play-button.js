$.fn.playButton = function(options) {
  var options = $.extend({
  }, options);

  if(/iPhone/i.test(navigator.userAgent)) {
    return;
  }

  $(this).each(function() {
    var video = $(this);

    var playButton = video.data("play-button");
    if (playButton == null) {
      playButton = $("<div class='play-button'></div>");
      playButton.html('&#9658;');
      playButton.data("video", video);
      $("body").append(playButton);
      video.data("play-button", playButton);
      playButton.click(function() {
        $(this).data("video")[0].play();
        $(this).hide();
      });
      video.click(function() {
        $(this).data("play-button").show();
        if (this.paused == true) {
          this.play();
        } else {
          this.pause();
        }
        return false;
      });
    }

    playButton.css(video.offset());
    playButton.css("width", video.width() +"px");
    playButton.css("height", video.height() +"px");
    playButton.css("line-height", video.height() +"px");
  });
};

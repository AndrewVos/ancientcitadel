$(document).on("click", ".gif", function() {
  if (this.paused == true) {
    this.play();
  } else {
    this.pause();
  }
});

$(function() {
  $(".gif").each(function() {
    var gif = $(this);
    gif.attr("poster", gif.data("poster"));
  });
});

$(function() {
  moveGifsAround();

  $("video.gif").on("play", function() {
    var $video = $(this);
    var elm = $video[0];

    var updateProgress = function() {
      var percentage = elm.currentTime / elm.duration;
      percentage = Math.round(percentage * 100) + 1;
      percentage = Math.min(percentage, 100);
      $video
        .parent()
        .find(".video-progress-inner")
        .css("width", percentage+"%");
      if (elm.paused == false) {
        setTimeout(updateProgress, 10);
      }
    };

    setTimeout(updateProgress, 10);
  });

  $(window).resize(moveGifsAround);
});

function gutter() {
  return 10;
}

function columns() {
  var width = (window.innerWidth > 0) ? window.innerWidth : screen.width;
  var columns = Math.round(width / 500);
  if (columns == 0) {
    columns = 1;
  }
  return columns;
}

function moveGifsAround() {
  var container = $(".items");
  var maximumWidth = Math.round(container.width() / columns());

  maximumWidth -= gutter();
  maximumWidth += (gutter() / columns());

  $(".item .gif").each(function() {
    var ratio = maximumWidth / $(this).data("width");
    $(this).height(ratio * $(this).data("height"));
  });

  $(".items").pack({
    columns: columns(),
    gutter: gutter(),
    selector: ".item"
  });
  $(".items .item").fadeTo( "fast" , 1);
}

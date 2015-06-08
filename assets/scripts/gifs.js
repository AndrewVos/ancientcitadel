$(document).on("click", ".gif", function() {
  if (this.paused == true) {
    this.play();
  } else {
    this.pause();
  }
});

function gifsPerRow() {
  var width = $(".items").width();
  if (width <= 960) {
    return 1;
  }

  var gifs = Math.round(width / 450);
  if (gifs == 0) {
    gifs = 1;
  }
  return gifs;
}

function calculateMaximumWidth() {
  var gutter = 5;
  var maximumWidth = 0;
  if (gifsPerRow() == 1) {
    maximumWidth = $(".items").width();
  } else {
    maximumWidth = $(".items").width() / gifsPerRow();
  }
  maximumWidth -= ((gifsPerRow() - 1) * gutter);
  return maximumWidth;
}

function moveGifsAround() {
  var maximumWidth = calculateMaximumWidth();

  $(".item").each(function() {
    var item = $(this);
    var gif = item.find(".gif");

    var width = gif.data("width");
    var height = gif.data("height");
    var ratio = maximumWidth / width;
    height = height * ratio;
    width = maximumWidth;
    gif.css("height", height);
    gif.css("width", width);
    item.css("width", gif.css("width"));
  });

  var $container = $('.items');
  $container.packery({
    itemSelector: '.item',
    gutter: 5
  });
}

$(moveGifsAround);

$(document).on("click", ".next-page a", function() {
  $(this).slideUp();
});

$(document).on("click", ".gif", function() {
  if (this.paused == true) {
    this.play();
  } else {
    this.pause();
  }
});

function moveGifsAround() {
  var maxWidth = $(document).width() / 4;
  maxWidth -= 10;

  $(".item").each(function() {
    var item = $(this);
    var gif = item.find(".gif");

    var width = gif.data("width");
    var height = gif.data("height");
    var ratio = maxWidth / width;
    height = height * ratio;
    width = maxWidth;
    gif.css("height", height);
    gif.css("width", width);
    item.css("width", gif.css("width"));
  });
}

function packery() {
  var $container = $('.items');
  $container.packery({
    itemSelector: '.item',
    gutter: 5
  });
}

$(document).on("click", ".gif", function() {
  if (this.paused == true) {
    this.play();
  } else {
    this.pause();
  }
});

$(function() {
  moveGifsAround();
  $(window).resize(moveGifsAround);
});

function gutter() {
  return 10;
}

function columns() {
  var containerWidth = $(".items").width();
  if (containerWidth <= 960) {
    return 1;
  }

  var columns = Math.round(containerWidth / 450);
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
}

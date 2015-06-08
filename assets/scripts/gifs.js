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
  // Measure maximum width before we make the items relative and absolute.
  // This ensures that we get the width with the scrollbar included.
  var maximumWidth = Math.round($(".items").width() / columns());

  $(".items").css("position", "relative");
  $(".items .item").css("position", "absolute");

  maximumWidth -= gutter();
  maximumWidth += (gutter() / columns());

  $(".item").width(maximumWidth);

  $(".item .gif").each(function() {
    var ratio = maximumWidth / $(this).data("width");
    $(this).height(ratio * $(this).data("height"));
  });

  var bottoms = [0, 0, 0, 0];

  var currentColumn = 0;
  $(".item").each(function() {
    var item = $(this);

    var top = bottoms[currentColumn];
    item.css("top",  top + "px");

    bottoms[currentColumn] = top + item.height() + gutter();

    var left = currentColumn * (maximumWidth + gutter());
    item.css("left", left+"px");

    currentColumn += 1;
    if (currentColumn == columns()) {
      currentColumn = 0;
    }
  });


  var maximumBottom = 0;
  $.each(bottoms, function(i, b) {
    if (b > maximumBottom) {
      maximumBottom = b;
    }
  });
  $(".items").css("height", maximumBottom+"px");
}

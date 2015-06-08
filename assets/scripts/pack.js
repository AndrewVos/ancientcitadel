$.fn.pack = function(options) {
  var options = $.extend({
    gutter: 10,
    columns: 4,
    selector: "div"
  }, options);

  // Measure maximum width before we make the items relative and absolute.
  // This ensures that we get the width with the scrollbar included.
  var container = $(this);

  var maximumWidth = Math.round(container.width() / options.columns);

  container.css("position", "relative");
  container.find(options.selector).css("position", "absolute");

  maximumWidth -= options.gutter;
  maximumWidth += (options.gutter / options.columns);

  $(options.selector).width(maximumWidth);

  var bottoms = [0, 0, 0, 0];

  var currentColumn = 0;
  $(options.selector).each(function() {
    var item = $(this);

    var top = bottoms[currentColumn];
    item.css("top",  top + "px");

    bottoms[currentColumn] = top + item.height() + options.gutter;

    var left = currentColumn * (maximumWidth + options.gutter);
    item.css("left", left+"px");

    currentColumn += 1;
    if (currentColumn == options.columns) {
      currentColumn = 0;
    }
  });

  var maximumBottom = 0;
  $.each(bottoms, function(i, b) {
    if (b > maximumBottom) {
      maximumBottom = b;
    }
  });
  container.css("height", maximumBottom+"px");
}


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

  $(options.selector).outerWidth(maximumWidth);

  var bottoms = [];
  for (i = 0; i < options.columns; i++) {
    bottoms.push(0);
  }

  $(options.selector).each(function() {
    var item = $(this);
    var shortestColumn = 0;
    for (i = 0; i < bottoms.length; i++) {
      if (bottoms[i] < bottoms[shortestColumn]) {
        shortestColumn = i;
      }
    }

    var top = bottoms[shortestColumn];
    item.css("top",  top + "px");

    bottoms[shortestColumn] = top + item.outerHeight() + options.gutter;

    var left = shortestColumn * (maximumWidth + options.gutter);
    item.css("left", left+"px");
  });

  var longestBottom = 0;
  for (i = 0; i < bottoms.length; i++) {
    if (bottoms[i] > longestBottom) {
      longestBottom = bottoms[i];
    }
  }
  container.css("height", longestBottom+"px");
}


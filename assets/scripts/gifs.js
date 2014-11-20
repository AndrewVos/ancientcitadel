$(document).on("click", ".item", function() {
  togglePlaying($(this).find("img"));
});

$(document).on("click", ".next-page a", function() {
  $(this).slideUp();
});

function togglePlaying(img) {
  var original = img.data("original");
  var item = img.parents(".item");

  if (img.attr("src") == original) {
    img.attr("src", img.data("preview"));
    item.css("opacity", 0.5);
  } else {
    img.data("preview", img.attr("src"));

    var complete = function() {
      item.css("opacity", 1);
      img.attr("src", original);
    };
    var image = new Image();
    image.onabort = complete;
    image.onerror = complete;
    image.onload = complete;
    setTimeout(function() {
      if (img.attr("src") != original) {
        img.parent().height(img.height())
        img.parent().width(img.width())
        img.attr("src", "");
        item.css("opacity", 1);
      }
    }, 100);
    image.src = original;
  }
}

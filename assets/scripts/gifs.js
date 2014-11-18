$(function() {
  loadPage();
});

function togglePlaying(img) {
  var original = img.data("original");
  var firstFrame = img.data("first-frame");
  if (img.attr("src") == original) {
    img.attr("src", firstFrame);
  } else {
    var complete = function() {
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
        img.attr("src", "/assets/images/loading.gif");
      }
    }, 100);
    image.src = original;
  }
}

function loadPage() {
  $(".item img").on("error", function() {
    $(this).parent().remove();
  });

  $(".item").click(function() {
    togglePlaying($(this).find("img"));
  });
}

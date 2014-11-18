$(function() {
  loadPage();
});

function togglePlaying(img) {
  var original = img.data("original");
  var loadingGif = "/assets/images/loading.gif";
  var item = img.parents(".item");

  if (img.attr("src") == original || img.attr("src") == loadingGif) {
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
        img.attr("src", loadingGif);
        item.css("opacity", 1);
      }
    }, 100);
    image.src = original;
  }
}

function loadPage() {
  var loadingGif = new Image();
  loadingGif.src = "/assets/images/loading.gif";

  $(".item").click(function() {
    togglePlaying($(this).find("img"));
  });
}

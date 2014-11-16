$(function() {
  loadPage();
});

function togglePlaying(img) {
  var original = img.data("original");
  var firstFrame = img.data("first-frame");
  if (img.attr("src") == original) {
    img.attr("src", firstFrame);
  } else {
    img.attr("src", original);
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

$(function() {
  loadPage();
});

function setAllToFirstFrameBut(img) {
  $(".item img").each(function() {
    $(this).attr("src", $(this).data("first-frame"));
  });
  if (img != null) {
    img.attr("src", img.data("original"));
  }
}

function loadPage() {
  $(".item img").on("error", function() {
    $(this).parent().remove();
  });

  $(".item").click(function() {
    setAllToFirstFrameBut($(this).find("img"));
  });
  $(".item").hover(function() {
    setAllToFirstFrameBut($(this).find("img"));
  }, function() {
    setAllToFirstFrameBut();
  });
}

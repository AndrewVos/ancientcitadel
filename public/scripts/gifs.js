$(function() {
  loadPage();
});

function setAllToFirstFrameBut(img) {
  $(".pin img").each(function() {
    $(this).attr("src", $(this).data("first-frame"));
  });
  if (img != null) {
    img.attr("src", img.data("original"));
  }
}

function loadPage() {
  $(".pin img").on("error", function() {
    $(this).parent().remove();
  });

  $(".pin").click(function() {
    setAllToFirstFrameBut($(this).find("img"));
  });
  $(".pin").hover(function() {
    setAllToFirstFrameBut($(this).find("img"));
  }, function() {
    setAllToFirstFrameBut();
  });
}

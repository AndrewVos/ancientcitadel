$(function() {
  moveGifsAround();
  plyr.setup({
    onSetup: moveGifsAround,
    controls: ["restart", "play", "current-time", "fullscreen"],
    click: true,
  });
  moveGifsAround();

  (function(d,p){
    var a=new XMLHttpRequest(),
    b=d.body;
    a.open("GET",p,!0);
    a.send();
    a.onload=function(){
      var c=d.createElement("div");
      c.style.display="none";
      c.innerHTML=a.responseText;
      b.insertBefore(c,b.childNodes[0])
    }
  })(document,"/assets/images/plyr-sprite.svg");

  $(window).resize(moveGifsAround);
});

function gutter() {
  return 10;
}

function columns() {
  var width = $(window).width();
  var columns = Math.round(width / 350);
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

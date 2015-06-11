function getCookie(cname) {
    var name = cname + "=";
    var ca = document.cookie.split(';');
    for(var i=0; i<ca.length; i++) {
        var c = ca[i];
        while (c.charAt(0)==' ') c = c.substring(1);
        if (c.indexOf(name) == 0) return c.substring(name.length,c.length);
    }
    return "";
}

function waitForTwitterToken() {
  if (checkForTwitterToken() == false) {
    setTimeout(waitForTwitterToken, 100);
  }
}

function checkForTwitterToken() {
  var t = getCookie("twitter_access_token");
  if (t == "") {
    return false;
  } else {
    $(".login-to-twitter").hide();
    $(".tweet").show();
    return true;
  }
}

$(function() {
  checkForTwitterToken();

  $(".login-to-twitter").click(waitForTwitterToken);

  $(".tweet").click(function() {
      var $tweet = $(this);
      var $status = $tweet.parent().find(".tweet-status");
      $status.text("Uploading...");
      $status.show();
      var gifId = $tweet.data("gif-id");
      $.ajax({
          url: "/tweet/" + gifId,
          type: "GET",
          success: function(data){
            $status.text("Cool, we put that on twitter for you.");
          },
          error: function(data) {
            $status.text("Aww man, something went wrong :( Could be because the gif was too big.");
          }
      });
      return false;
  });
});

$(function() {
  var iOS = /(iPad|iPhone|iPod)/g.test(navigator.userAgent);
  if (iOS) {
    $(document).on("click", ".ios-intent", function() {
      var intent = $(this);
      var now = new Date().valueOf();
      setTimeout(function() {
        if (new Date().valueOf() - now > 100) {
          return;
        }
        window.location = intent.attr("href");
      }, 25);
      window.location = intent.data("ios-intent");
      return false;
    });
  }
});

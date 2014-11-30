$(document).on("click", ".next-page a", function() {
  $(this).slideUp();
});

$(document).on("click", ".gif", function() {
  if (this.paused == true) {
    this.play();
  } else {
    this.pause();
  }
});

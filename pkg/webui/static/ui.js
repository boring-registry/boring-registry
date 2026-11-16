// Progressive enhancements for the boring-registry web UI.
document.documentElement.classList.add("js");

document.addEventListener("click", function (event) {
  var button = event.target.closest("[data-copy]");
  if (!button || !navigator.clipboard) {
    return;
  }
  var code = button.closest(".card").querySelector("pre code");
  if (!code) {
    return;
  }
  navigator.clipboard.writeText(code.textContent).then(function () {
    button.textContent = "Copied";
    button.classList.add("copied");
    setTimeout(function () {
      button.textContent = "Copy";
      button.classList.remove("copied");
    }, 1500);
  });
});

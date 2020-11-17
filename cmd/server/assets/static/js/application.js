$(function() {
  // Add data-toogle="tooltop" to toggle tooltips!
  $('[data-toggle="tooltip"]').tooltip();

  // Add data-submit-form properties to a link to have it act as a submit
  // button. You can also add a data-confirm attribute with a confirmation
  // prompt.
  $("a[data-submit-form]").click(function(e) {
    e.preventDefault();

    let $this = $(e.currentTarget);
    let confirm = $this.data("confirm");
    if (typeof confirm !== "undefined") {
      if (window.confirm(confirm) !== true) {
        return;
      }
    }

    $this.closest("form").submit();
  });

  // Add data-method to a link and make the resulting href submit as that
  // method. You can also add a data-confirm attribute with a confirmation
  // prompt.
  $("a[data-method]").click(function(e) {
    e.preventDefault();

    let $this = $(e.currentTarget);
    let confirm = $this.data("confirm");

    if (typeof confirm !== "undefined") {
      if (window.confirm(confirm) !== true) {
        return;
      }
    }

    let csrfToken = $("meta[name=csrf-token]").attr("content");
    let $csrfField = $("<input>")
      .attr("type", "hidden")
      .attr("name", "gorilla.csrf.Token")
      .attr("value", csrfToken);

    let $inputField = $("<input>")
      .attr("type", "hidden")
      .attr("name", "_method")
      .attr("value", $this.data("method"));

    let $form = $("<form>")
      .attr("method", "POST")
      .attr("action", $this.attr("href"));

    $form.append($csrfField);
    $form.append($inputField);

    $form.appendTo("body").submit();
  });

  // Add data-toggle-password to an element with the value pointing to the id
  // of an input[type="password"]. It will toggle/untoggle the value.
  $("a[data-toggle-password]").click(function(e) {
    e.preventDefault();

    let $this = $(e.currentTarget);
    let selector = $this.data("togglePassword");
    let $input = $("#" + selector);
    let $icon = $this.find("span.oi");

    if ($input.attr("type") == "password") {
      $input.attr("type", "text");
      $icon.addClass("oi-lock-unlocked");
      $icon.removeClass("oi-lock-locked");
    } else if ($input.attr("type") == "text") {
      $input.attr("type", "password");
      $icon.addClass("oi-lock-locked");
      $icon.removeClass("oi-lock-unlocked");
    }
  });

  $("a[data-fill-target]").click(function(e) {
    e.preventDefault();

    let $this = $(e.currentTarget);
    let selector = $this.data("fillTarget");

    let value = $this.data("fillValue");
    let $target = $("#" + selector);
    $target.val(value);
  });

  $("a[data-copy]").click(function(e) {
    e.preventDefault();

    let $this = $(e.currentTarget);
    let selector = $this.data("copyTarget");
    let $target = $("#" + selector);

    $target[0].focus();
    $target[0].setSelectionRange(0, 99999);

    document.execCommand("copy");
    $this.tooltip("hide");
    document.getSelection().removeAllRanges();
  });

  $("[data-timestamp]").each(function(i, e) {
    let $this = $(e);
    let date = new Date($this.data("timestamp"));

    let year = date.getFullYear();
    let month = date.getMonth() + 1;
    if (month < 10) {
      month = `0${month}`;
    }
    let day = date.getDate();
    if (day < 10) {
      day = `0${day}`;
    }
    let ampm = "AM";
    let hours = date.getHours();
    if (hours > 12) {
      ampm = "PM";
      hours = hours - 12;
    }
    if (hours < 10) {
      hours = `0${hours}`;
    }
    let minutes = date.getMinutes();
    if (minutes < 10) {
      minutes = `0${minutes}`;
    }

    $this.html(`${year}-${month}-${day} ${hours}:${minutes} ${ampm}`);
  });

  // Toast shows alerts/flash messages.
  $(".toast").toast("show");

  // Flash is the flash handler
  let flash = (function() {
    let $alerts = $("#alerts-container");

    let f = {};

    // clear clears any existing flashes.
    f.clear = function() {
      $alerts.empty();
    };

    // error creates a flash error message.
    f.error = function(message) {
      f.flash("error", message);
    };

    // warning creates a flash warning message.
    f.warning = function(message) {
      f.flash("warning", message);
    };

    // alert creates a flash notice message.
    f.alert = function(message) {
      f.flash("alert", message);
    };

    // flash is a lower-level function for generating a flash message. Usually
    // you want to call flash.alert() or flash.error() instead.
    f.flash = function(level, message) {
      let headerClass;
      let headerIconClass;
      let headerText;

      switch (level) {
        case "error":
          headerClass = "text-danger";
          headerIconClass = "oi-circle-x";
          headerText = "Error";
          break;
        case "warning":
          headerClass = "text-warning";
          headerIconClass = "oi-warning";
          headerText = "Warning";
          break;
        case "alert":
          headerClass = "text-info";
          headerIconClass = "oi-info";
          headerText = "Notice";
          break;
        default:
          throw `invalid level ${level}`;
      }

      let $toast = $("<div>");
      $toast.addClass("toast bg-white");
      $toast.attr("role", "alert");
      $toast.attr("aria-live", "assertive");
      $toast.attr("aria-atomic", "true");

      // Auto-dismiss notices, but everything else is sticky.
      if (level === "alert") {
        $toast.attr("data-delay", 10000);
        $toast.attr("data-autohide", true);
      } else {
        $toast.attr("data-autohide", false);
      }

      // Create the header.
      let $header = $("<div>");
      $header.addClass("toast-header");
      $header.addClass(headerClass);

      // Create the icon.
      let $icon = $("<span>");
      $icon.addClass("oi mr-2");
      $icon.addClass(headerIconClass);
      $icon.attr("aria-hidden", true);
      $header.append($icon);

      // Create the text next to the icon.
      let $headerText = $("<strong>");
      $headerText.addClass("mr-auto");
      $headerText.html(headerText);
      $header.append($headerText);

      // Create the close button.
      let $button = $("<button>");
      $button.addClass("ml-2 mb-1 close");
      $button.attr("type", "button");
      $button.attr("data-dismiss", "toast");
      $button.attr("aria-label", "Close");
      $button.html('<span aria-hidden="true">&times;</span>');
      $header.append($button);

      // Add the header to the toast.
      $toast.append($header);

      // Create the body.
      let $body = $("<div>");
      $body.addClass("toast-body");
      $body.html(message);

      // Add the body to the toast.
      $toast.append($body);

      // Add the toast to the page.
      $alerts.append($toast);

      // Show the toast.
      $toast.toast("show");
    };

    return f;
  })();

  window.flash = flash;
});


function setCookie(cname, cvalue, exdays) {
  var d = new Date();
  d.setTime(d.getTime() + (exdays * 24 * 60 * 60 * 1000));
  var expires = "expires=" + d.toUTCString();
  document.cookie = cname + "=" + cvalue + ";" + expires;
}

function getCookie(cname) {
  var name = cname + "=";
  var ca = document.cookie.split(';');
  for (var i = 0; i < ca.length; i++) {
    var c = ca[i];
    while (c.charAt(0) == ' ') {
      c = c.substring(1);
    }
    if (c.indexOf(name) == 0) {
      return c.substring(name.length, c.length);
    }
  }
  return "";
}

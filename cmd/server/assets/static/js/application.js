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
    $this.tooltip({
      placement: "top",
      title: date.toISOString(),
    });
    $this.text(date.toLocaleString());
  });

  // Disable propagation on links in menus if they are marked as such.
  $(document).on('click', 'div.dropdown-menu .keep-open', function(e) {
    e.stopPropagation();
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
      $headerText.text(headerText);
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
      $body.text(message);

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
  let d = new Date();
  d.setTime(d.getTime() + (exdays * 24 * 60 * 60 * 1000));
  let expires = "expires=" + d.toUTCString();
  document.cookie = cname + "=" + cvalue + ";" + expires;
}

function getCookie(cname) {
  let name = cname + "=";
  let ca = document.cookie.split(';');
  for (let i = 0; i < ca.length; i++) {
    let c = ca[i];
    while (c.charAt(0) == ' ') {
      c = c.substring(1);
    }
    if (c.indexOf(name) == 0) {
      return c.substring(name.length, c.length);
    }
  }
  return "";
}

// Common error codes which should cancel the whole upload.
const stopUploadingCodes = [
  '403', // forbidden
  '404', // not-found
  '503', // unavailable
];

const stopUploadingEnum = [
  'maintenance_mode',
  'sms_queue_full',
];

async function uploadWithRetries(uploadFn) {
  let cancel = false;
  for (let retries = 3; retries > 0; retries--) {
    await uploadFn().then(
      () => { retries = 0; }).catch(
        async function(err) {
          if (!err) {
            return;
          }
          if (err.responseJSON && stopUploadingEnum.includes(err.responseJSON.errorCode)) {
            flash.alert("Status " + err.responseJSON.errorCode + " detected. Canceling remaining upload.");
            cancel = true;
            retries = 0;
          } else if (stopUploadingCodes.includes(err.status)) {
            flash.alert("Code " + err.status + " detected. Canceling remaining upload.");
            cancel = true;
            retries = 0;
          } else {
            // Throttling
            let after = err.getResponseHeader("retry-after");
            if (after) {
              let sleep = new Date(after) - new Date();
              if (sleep > 0) {
                flash.alert("Rate limited. Sleeping for " + ((sleep + 100) / 1000) + "s.");
                await new Promise(r => setTimeout(r, sleep + 100));
              }
            } else {
              retries = 0;
            }
          }
        });
  }
  return cancel;
}

function checkPasswordValid(pwd, retype, requirements) {
  let valid = true;

  if (pwd != retype) {
    decorateValid($('#retyped'));
    valid = false;
  } else {
    decorateInvalid($('#retyped'));
  }

  if (requirements) {
    let upper = 0;
    let lower = 0;
    let digit = 0;
    let special = 0;
    let specialPattern = new RegExp(/[~`!#$%\^&*+=\-\[\]\\';,/{}|\\":<>\?]/);
    for (let i = 0; i < pwd.length; i++) {
      let c = pwd.charAt(i);
      if (!isNaN(parseInt(c, 10))) {
        digit++;
      } else if (specialPattern.test(c)) {
        special++;
      } else if (c == c.toUpperCase()) {
        upper++;
      } else if (c == c.toLowerCase()) {
        lower++;
      }
    }

    if (pwd.length < requirements.Length) {
      decorateValid($('#length-req'));
      valid = false;
    } else {
      decorateInvalid($('#length-req'));
    }

    if (upper < requirements.Uppercase) {
      decorateValid($('#upper-req'));
      valid = false;
    } else {
      decorateInvalid($('#upper-req'));
    }

    if (lower < requirements.Lowercase) {
      decorateValid($('#lower-req'));
      valid = false;
    } else {
      decorateInvalid($('#lower-req'));
    }

    if (digit < requirements.Number) {
      decorateValid($('#num-req'));
      valid = false;
    } else {
      decorateInvalid($('#num-req'));
    }

    if (special < requirements.Special) {
      decorateValid($('#special-req'));
      valid = false;
    } else {
      decorateInvalid($('#special-req'));
    }
  }

  return valid;
}

const errClass = "oi oi-circle-x pr-1";
const checkClass = "oi oi-circle-check pr-1";

function decorateValid($element) {
  $element.find("#icon").attr("class", errClass);
  $element.addClass("text-danger");
  $element.removeClass("text-muted");
}

function decorateInvalid($element) {
  $element.find("#icon").attr("class", checkClass);
  $element.addClass("text-muted");
  $element.removeClass("text-danger");
}

function loginScripts(hasCurrentUser, onLoginSuccess) {
  let $loginDiv = $('#login-div');
  let $submit = $('#submit');
  let $loginForm = $('#login-form');
  let $email = $('#email');
  let $password = $('#password');

  let $pinDiv = $('#sms-code-div');
  let $pinText = $('#code-text');
  let $pinClose = $('#sms-code-close');
  let $pinForm = $('#sms-code-form');
  let $pin = $('#sms-code');
  let $submitPin = $('#sms-code-submit');
  let $resendPin = $('#sms-code-resend');
  let $smsChange = $('#sms-change');

  let $registeredDiv = $('#registered-div');
  let $factors = $('#factors');

  let verId = "";
  let selectedFactorIndex = 0;

  window.recaptchaVerifier = new firebase.auth.RecaptchaVerifier(
    'recaptcha-container', {
    'size': 'invisible',
    'expired-callback': e => {
      window.recaptchaVerifier.reset();
    },
    'error-callback': e => {
      window.recaptchaVerifier.reset();
    },
  });

  $loginForm.on('submit', function(event) {
    event.preventDefault();
    onSignInSubmit();
  });

  $pinForm.on('submit', function(event) {
    event.preventDefault();

    // Disable the submit button so we only attempt once.
    $submitPin.prop('disabled', true);

    // Ask user for the SMS verification code.
    let cred = firebase.auth.PhoneAuthProvider.credential(verId, $pin.val().trim());
    let multiFactorAssertion = firebase.auth.PhoneMultiFactorGenerator.assertion(cred);
    // Complete sign-in.
    resolver.resolveSignIn(multiFactorAssertion)
      .then(function(userCredential) {
        onLoginSuccess();
      }).catch(function(err) {
        flash.clear();
        flash.error(err.message);
        window.recaptchaVerifier.reset();
        $submitPin.prop('disabled', false);
      });
  });

  $pinClose.on('click', function(event) {
    event.preventDefault();
    $submit.prop('disabled', false);
    $factors.empty();
    $loginDiv.show();
    $pinDiv.addClass('d-none');
  });

  $resendPin.on('click', function(event) {
    event.preventDefault();
    resendPin();
  });

  $smsChange.on('click', function(event) {
    event.preventDefault();
    $pinDiv.addClass('d-none');
    $registeredDiv.removeClass('d-none');
  });

  function onSignInSubmit() {
    // Disable the submit button so we only attempt once.
    $submit.prop('disabled', true);

    let signInPromise;
    if (hasCurrentUser) {
      let credentials = firebase.auth.EmailAuthProvider.credential($email.val().trim(), $password.val());
      signInPromise = firebase.auth().currentUser.reauthenticateWithCredential(credentials);
    } else {
      signInPromise = firebase.auth().signInWithEmailAndPassword($email.val(), $password.val());
    }

    signInPromise.then(function(userCredential) {
      onLoginSuccess();
    }).catch(function(error) {
      if (error.code == 'auth/multi-factor-auth-required') {
        window.recaptchaVerifier.render();
        resolver = error.resolver;
        populatePinText(resolver.hints);
        populateFactors(resolver.hints);
        if (resolver.hints[selectedFactorIndex].factorId === firebase.auth.PhoneMultiFactorGenerator.FACTOR_ID) {
          let phoneInfoOptions = {
            multiFactorHint: resolver.hints[selectedFactorIndex],
            session: resolver.session
          };
          let phoneAuthProvider = new firebase.auth.PhoneAuthProvider();
          let appVerifier = window.recaptchaVerifier;
          return phoneAuthProvider.verifyPhoneNumber(phoneInfoOptions, appVerifier)
            .then(function(verificationId) {
              verId = verificationId;
              setTimeout(function() { $resendPin.removeClass('disabled'); }, 15000);
              $submitPin.prop('disabled', false);
              $loginDiv.hide();
              $pinDiv.removeClass('d-none');
            }).catch(function(error) {
              window.recaptchaVerifier.reset();
              flash.clear();
              flash.error(error);
              $submit.prop('disabled', false);
            });
        } else {
          flash.clear();
          flash.error('Unsupported 2nd factor authentication type.');
        }
      } else if (error.code == 'auth/too-many-requests') {
        flash.clear();
        flash.error(error.message);
        $submit.prop('disabled', false);
      } else {
        console.error(error);
        flash.clear();
        flash.error("Sign-in failed. Please try again.");
        $submit.prop('disabled', false);
      }
    });
  }

  function resendPin() {
    $resendPin.addClass('disabled');
    setTimeout(function() { $resendPin.removeClass('disabled'); }, 15000);

    let phoneInfoOptions = {
      multiFactorHint: resolver.hints[selectedFactorIndex],
      session: resolver.session
    };
    populatePinText(resolver.hints);
    let phoneAuthProvider = new firebase.auth.PhoneAuthProvider();
    let appVerifier = window.recaptchaVerifier;
    phoneAuthProvider.verifyPhoneNumber(phoneInfoOptions, appVerifier)
      .then(function(verificationId) {
        verId = verificationId;
      }).catch(function(error) {
        window.recaptchaVerifier.reset();
        flash.clear();
        flash.error(error.message);
        $submit.prop('disabled', false);
      });
  }

  function populatePinText(hints) {
    let $displayName = $('<span/>');
    $displayName.addClass('text-info');
    $displayName.text(hints[selectedFactorIndex].displayName);

    $pinText.empty();
    $pinText.text('Code sent to ');
    $pinText.append($displayName);
  }

  function populateFactors(hints) {
    if (hints.length > 0) {
      for (i = 0; i < hints.length; i++) {
        appendAuthFactor(hints[i], i);
      }
    }
    if (hints.length > 1) {
      $smsChange.removeClass("d-none");
    }
  }

  function appendAuthFactor(factor, i) {
    let $li = $('<a/>');
    $li.addClass('list-group-item list-group-item-action');
    if (i == 0) {
      $li.addClass('bg-light');
      $li.attr('id', 'selected-factor');
    }
    let $row = $('<div/>').text(factor.displayName);
    $li.append($row);

    let $icon = $('<span/>');
    $icon.addClass('oi oi-phone mr-1');
    $icon.attr('aria-hidden', 'true');
    $row.prepend($icon);

    let $time = $('<small/>');
    $time.addClass('row text-muted ml-1');
    $time.text(`Phone number: ${factor.phoneNumber}`);
    $row.append($time);

    $li.on('click', function(event) {
      $registeredDiv.addClass('d-none');
      $pinDiv.removeClass('d-none');
      if (selectedFactorIndex == i) {
        return;
      }

      $('#selected-factor').removeClass('bg-light');
      $li.addClass('bg-light');
      $li.attr('id', 'selected-factor');
      selectedFactorIndex = i;
      resendPin();
    });

    $factors.append($li);
  }
}

// generates a random alphanumeric code
function genRandomString(len) {
  let i = len;
  let s = "";
  for (; i >= 6; i -= 6) {
    s += Math.random().toString(36).substr(2, 8);
  }
  if (i > 0) {
    s += Math.random().toString(36).substr(2, 2 + i);
  }
  return s;
}

// element is expected to be a jquery element or dom query selector, ts is
// the number of seconds since epoch, UTC.
function countdown(element, ts, expiredCallback) {
  if (typeof (ts) === 'undefined') {
    return;
  }

  let $element = $(element);
  let date = new Date(ts * 1000).getTime();

  const formattedTime = function() {
    let now = new Date().getTime();
    let diff = date - now;

    if (diff <= 0) {
      return false;
    }

    let hours = Math.floor(diff / (1000 * 60 * 60));
    let minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));
    let seconds = Math.floor((diff % (1000 * 60)) / 1000);

    let time;

    // hours
    if (hours < 10) {
      time = `0${hours}`;
    } else {
      time = `${hours}`;
    }

    // minutes
    if (minutes < 10) {
      time = `${time}:0${minutes}`;
    } else {
      time = `${time}:${minutes}`;
    }

    // seconds
    if (seconds < 10) {
      time = `${time}:0${seconds}`;
    } else {
      time = `${time}:${seconds}`;
    }

    return time;
  };

  // Fire once so the time is displayed immediately.
  setTimeOrExpired($element, formattedTime(), expiredCallback);

  // Set timer.
  const fn = setInterval(function() {
    let time = formattedTime();
    if (!time) {
      clearInterval(fn);
    }
    setTimeOrExpired($element, time, expiredCallback);
  }, 1000);

  return fn;
}

function setTimeOrExpired(element, time, expiredCallback) {
  let $element = $(element);

  if (!time) {
    if (typeof expiredCallback === 'function') {
      expiredCallback();
    }

    let expiredText = $element.data("countdownExpired");
    if (!expiredText) {
      expiredText = 'EXPIRED';
    }
    return element.html(expiredText);
  }

  let prefix = $element.data("countdownPrefix");
  if (!prefix) {
    prefix = '';
  }
  return element.html(`${prefix} ${time}`.trim());
}

// utcDate parses the given RFC-3339 date as a javascript date, then converts it
// to a UTC date.
function utcDate(str) {
  let d = new Date(str);
  let offset = d.getTimezoneOffset() * 60 * 1000;
  return new Date(d.getTime() + offset);
}

const batchSize = 10;
const showMaxResults = 50;

function initBulkUploadUI() {
  $form = $('#form');
  $csv = $('#csv');
  $fileLabel = $('#file-label');
  $import = $('#import');
  $cancel = $('#cancel');
  $table = $('#csv-table');
  $tableBody = $('#csv-table-body');
  $progressDiv = $('#progress-div');
  $progress = $('#progress');
  $retryCode = $('#retry-code');
  $rememberCode = $('#remember-code');
  $inputSMSTemplate = $('select#sms-template');
  $newCode = $('#new-code');
  $startAt = $('#start-at');

  $receiptDiv = $('#receipt-div');
  $save = $('#save');
  $receiptSuccess = $('#receipt-success');
  $receiptFailure = $('#receipt-failure');

  $errorDiv = $('#error-div');
  $errorTable = $('#error-table');
  $errorTableBody = $('#error-table > tbody');
  $errorTooMany = $('#error-too-many');

  $successDiv = $('#success-div');
  $successTable = $('#success-table');
  $successTableBody = $('#success-table > tbody');
  $successTooMany = $('#success-too-many');

  let now = new Date();
  $save.attr('download', `${now.toISOString().split('T')[0]}-bulk-issue-log.csv`);
}

function resetBulkUploadUI() {
  $import.prop('disabled', true);
  $cancel.removeClass('d-none');

  $table.removeClass('d-none');
  $progressDiv.removeClass('d-none');

  $receiptDiv.addClass('d-none');
  $save.attr("href", "data:text/plain,");
  $receiptSuccess.text(0);
  $receiptFailure.text(0);

  $errorTooMany.addClass('d-none');
  $errorDiv.addClass("d-none");
  $errorTableBody.empty();

  $successTooMany.addClass('d-none');
  $successDiv.addClass("d-none");
  $successTableBody.empty();
}

function readBulkUploadCSVFile() {
  // State for managing cleanup and canceling
  let cancelUpload = false;
  let cancel = () => {
    cancelUpload = true;
  };

  let start = async function(e) {
    let retryCode = $retryCode.val();
    let template = $inputSMSTemplate.val();
    let rows = e.target.result.split('\n');
    let batch = [];
    let batchLines = [];
    total = 0;
    totalErrs = 0;
    $tableBody.empty();

    for (let i = parseInt($startAt.val() - 1); i < rows.length && !cancelUpload; i++) {
      // Clear batch that was just uploaded.
      if (batch.length >= batchSize) {
        batch = [];
        batchLines = [];
      }

      // Add to batch if the next row is valid.
      let request = buildBatchIssueRequest(rows[i], retryCode, template, i + 1);
      if (request != "") {
        batch.push(request);
        batchLines.push(i + 1);
      }

      // If we've hit the batch limit or end of file, upload it.
      if (batch.length >= batchSize || i == rows.length - 1 && batch.length > 0) {
        $tableBody.empty();
        for (let r = 0; r < batch.length; r++) {
          let $row = $('<tr/>');
          $row.append($('<td/>').text(batch[r]["phone"]));
          $row.append($('<td/>').text(batch[r]["testDate"]));
          $tableBody.append($row);
        }

        cancelUpload = await uploadWithRetries(() => uploadBatchIssue(batch, batchLines));

        if (cancelUpload) {
          if (total > 0) {
            flash.warning(`Successfully issued ${total} codes. ${(rows.length - i)} +  remaining.`);
          }
          break;
        }
        $startAt.val(i + 1);
        let percent = Math.floor((i + 1) * 100 / rows.length) + "%";
        $progress.width(percent);
        $progress.html(percent);
      }
    }

    $save.attr("href", $save.attr("href") + '\n');

    if (!cancelUpload) {
      $progress.width('100%');
      $progress.html('100%');
      if (total > 0) {
        flash.alert(`Successfully issued ${total} codes.`);
      }
    }

    if (totalErrs > 0) {
      flash.error(`Received errors for ${totalErrs} entries. See error table for details.`);
    }

    $import.prop('disabled', false);
    $cancel.addClass('d-none');
    $table.addClass('d-none');
    $tableBody.empty();
  };

  return { start, cancel };
}

function buildBatchIssueRequest(thisRow, retryCode, template, line) {
  thisRow = thisRow.trim();
  if (thisRow == "") {
    return "";
  }
  let request = {};
  let cols = thisRow.split(',');

  // Escape csv row contents
  request["phone"] = $("<div>").text(cols[0].trim()).html();
  request["testDate"] = (cols.length > 1) ? $("<div>").text(cols[1].trim()).html() : "";
  request["symptomDate"] = (cols.length > 2) ? $("<div>").text(cols[2].trim()).html() : "";
  request["testType"] = (cols.length > 3) ? $("<div>").text(cols[3].trim()).html() : "confirmed";

  if (request["testType"] == "") {
    request["testType"] = "confirmed";
  }

  // Skip missing phone number
  if (request["phone"] == "" || cols.Length < 2) {
    let code = {
      errorCode: "invalid_client",
      error: "phone number missing",
    };
    showErroredCode(request, code, line);
    return "";
  }

  let uuid = "";
  if (cols.length > 6) {
    uuid = $("<div>").text(cols[6].trim()).html();
  }
  if (uuid.length != 36) {
    // Generate a UUID by hashing phone
    let hs = String(CryptoJS.HmacSHA256(request["phone"], retryCode)).substr(0, 36);
    uuid = hs.substr(0, 8) + '-' + hs.substr(9, 4) + '-' + hs.substr(13, 4) + '-' + hs.substr(17, 4) + '-' + hs.substr(21, 12);
  }

  request["uuid"] = uuid;
  request["smsTemplateLabel"] = template;
  request["tzOffset"] = tzOffset;

  // CSV file has error codes in the file. Usually means a retry of the receipt file.
  // Skip un-retryable errors
  if (cols.length >= 8) {
    let errCode = $("<div>").text(cols[7].trim()).html();
    if (errCode == "success") {
      let code = {
        errorCode: errCode,
        error: `code uuid ${uuid} already succeeded. skipping code.`,
      };
      showErroredCode(request, code, line);
      return "";
    } else if (errCode == "uuid_already_exists") {
      let existingUUID = $("<div>").text(cols[6].trim()).html();
      if (uuid == existingUUID) {
        let code = {
          errorCode: errCode,
          error: `code uuid ${existingUUID} already exists on the server. skipping code.`,
        };
        showErroredCode(request, code, line);
        return "";
      }
    }
  }
  return request;
}

function uploadBatchIssue(data, lines) {
  let req = {
    'codes': data,
    // Request is padded with 5-15 random chars. These are ignored but vary the size of the request
    // to prevent network traffic observation.
    'padding': btoa(genRandomString(5 + Math.floor(Math.random() * 15)))
  };
  return $.ajax({
    url: '/codes/batch-issue',
    type: 'POST',
    dataType: 'json',
    cache: false,
    contentType: 'application/json',
    headers: { 'X-CSRF-Token': csrfToken },
    data: JSON.stringify(req),
    success: function(result) {
      if (!result.responseJSON || !result.responseJSON.codes) {
        return;
      }
      readCodesBatch(data, lines, result.responseJSON.codes);
    },
    error: function(xhr, resp, text) {
      if (!xhr || !xhr.responseJSON) {
        return;
      }

      if (!xhr.responseJSON.codes) {
        let message = resp;
        if (xhr.responseJSON.error) {
          message = message + ": " + xhr.responseJSON.error;
        }
        flash.error(message);
        return;
      }
      readCodesBatch(data, lines, xhr.responseJSON.codes);
    },
  });
}

function readCodesBatch(data, lines, codes) {
  for (let i = 0; i < codes.length; i++) {
    let code = codes[i];
    if (code.error) {
      showErroredCode(data[i], code, lines[i]);
    } else {
      showSuccessfulCode(data[i], code, lines[i]);
    }
  }
}

function showErroredCode(request, code, line) {
  // We show error for already-succeeded codes. Skip those for the count.
  if (code.errorCode != "success") {
    totalErrs++;
  }
  $receiptFailure.text(totalErrs);
  if (totalErrs == 1) {
    $receiptDiv.removeClass('d-none');
    $errorDiv.removeClass('d-none');
  }
  if (totalErrs == showMaxResults + 1) {
    $errorTableBody.empty();
    $errorTable.addClass('d-none');
    $errorTooMany.removeClass('d-none');
  }
  $save.attr("href", `${$save.attr("href")}${request["phone"]},${request["testDate"]},${request["symptomDate"]},${request["testType"]},,,${request["uuid"]},${code.errorCode},${code.error}\n`);
  if (totalErrs > showMaxResults) {
    return;
  }

  let $row = $('<tr/>');
  $row.append($('<td/>').text(line));
  $row.append($('<td/>').text(request["phone"]));
  $row.append($('<td/>').text(request["testDate"]));
  $row.append($('<td/>').text(code.error));
  $errorTableBody.append($row);
}

function showSuccessfulCode(request, code, line) {
  total++;
  $receiptSuccess.text(total);
  if (total == 1) {
    $receiptDiv.removeClass('d-none');
    $successDiv.removeClass('d-none');
    $successTable.removeClass('d-none');
  }
  if (total == showMaxResults + 1) {
    $successTableBody.empty();
    $successTable.addClass('d-none');
    $successTooMany.removeClass('d-none');
  }
  $save.attr("href", `${$save.attr("href")}${request["phone"]},${request["testDate"]},${request["symptomDate"]},${request["testType"]},,,${code.uuid},success\n`);
  if (total > showMaxResults) {
    return;
  }

  let $row = $('<tr/>');
  $row.append($('<td/>').text(line));
  $row.append($('<td/>').text(request["phone"]));
  $row.append($('<td/>').text(request["testDate"]));
  $row.append($('<td/>').text(code.uuid));
  $successTableBody.append($row);
}

function redrawCharts(chartsData, timeout) {
  let redrawPending = false;
  let windowWidth = 0;
  $(window).resize(function() {
    let w = $(window).width();
    if (w != windowWidth) {
      windowWidth = w;
    } else {
      return;
    }

    if (!redrawPending) {
      redrawPending = true;
      setTimeout(function() {
        redraw();
        redrawPending = false;
      }, timeout);
    }
  });

  function redraw() {
    let c;
    for (c of chartsData) {
      if (c.options) {
        c.options.animation = null;
      }
      c.chart.draw(c.data, c.options);
    }
  }
}

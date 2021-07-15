window.addEventListener('load', (event) => {
  // Add data-toogle="tooltip" to toggle tooltips!
  let tooltips = document.querySelectorAll('[data-bs-toggle="tooltip"]');
  [].forEach.call(tooltips, (element) => {
    new bootstrap.Tooltip(element);
  });

  // Add data-submit-form properties to a link to have it act as a submit
  // button. You can also add a data-confirm attribute with a confirmation
  // prompt.
  $('a[data-submit-form]').click(function (e) {
    e.preventDefault();

    let $this = $(e.currentTarget);
    let confirm = $this.data('confirm');
    if (typeof confirm !== 'undefined') {
      if (window.confirm(confirm) !== true) {
        return;
      }
    }

    $this.closest('form').submit();
  });

  // Add data-method to a link and make the resulting href submit as that
  // method. You can also add a data-confirm attribute with a confirmation
  // prompt.
  $('a[data-method]').click(function (e) {
    e.preventDefault();

    let $this = $(e.currentTarget);
    let confirm = $this.data('confirm');

    if (typeof confirm !== 'undefined') {
      if (window.confirm(confirm) !== true) {
        return;
      }
    }

    let csrfToken = getCSRFToken();
    let $csrfField = $('<input>').attr('type', 'hidden').attr('name', 'csrf_token').attr('value', csrfToken);

    let $inputField = $('<input>').attr('type', 'hidden').attr('name', '_method').attr('value', $this.data('method'));

    let $form = $('<form>').attr('method', 'POST').attr('action', $this.attr('href'));

    $form.append($csrfField);
    $form.append($inputField);

    $form.appendTo('body').submit();
  });

  $('a[data-fill-target]').click(function (e) {
    e.preventDefault();

    let $this = $(e.currentTarget);
    let selector = $this.data('fillTarget');

    let value = $this.data('fillValue');
    let $target = $('#' + selector);
    $target.val(value);
  });

  $('a[data-copy]').click(function (e) {
    e.preventDefault();

    let $this = $(e.currentTarget);
    let selector = $this.data('copyTarget');
    let $target = $('#' + selector);

    $target[0].focus();
    $target[0].setSelectionRange(0, 99999);

    document.execCommand('copy');
    $this.tooltip('hide');
    document.getSelection().removeAllRanges();
  });

  $('[data-timestamp]').each(function (i, e) {
    let $this = $(e);
    let date = new Date($this.data('timestamp'));
    $this.tooltip({
      placement: 'top',
      title: date.toISOString(),
    });
    $this.text(date.toLocaleString());
  });
});

window.addEventListener('DOMContentLoaded', () => {
  // Disable all interactive elements if the page is disabled.
  if (document.querySelector('body.disabled-controls') !== null) {
    document.querySelectorAll('input, button, select, textarea, a.btn').forEach(async (element) => {
      element.classList.add('disabled');
      element.classList.add('readonly');
      element.disabled = true;
      element.readonly = true;
    });
  }
});

function getCSRFToken() {
  return document.querySelector('meta[name="csrf-token"]').content;
}

function setCookie(cname, cvalue, exdays) {
  let d = new Date();
  d.setTime(d.getTime() + exdays * 24 * 60 * 60 * 1000);
  let expires = 'expires=' + d.toUTCString();
  document.cookie = cname + '=' + cvalue + ';' + expires;
}

function getCookie(cname) {
  let name = cname + '=';
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
  return '';
}

function checkPasswordValid(pwd, retype, requirements) {
  let valid = true;

  if (pwd && pwd.length > 0 && pwd == retype) {
    decorateValid($('#retyped'));
  } else {
    decorateInvalid($('#retyped'));
    valid = false;
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
      decorateInvalid($('#length-req'));
      valid = false;
    } else {
      decorateValid($('#length-req'));
    }

    if (upper < requirements.Uppercase) {
      decorateInvalid($('#upper-req'));
      valid = false;
    } else {
      decorateValid($('#upper-req'));
    }

    if (lower < requirements.Lowercase) {
      decorateInvalid($('#lower-req'));
      valid = false;
    } else {
      decorateValid($('#lower-req'));
    }

    if (digit < requirements.Number) {
      decorateInvalid($('#num-req'));
      valid = false;
    } else {
      decorateValid($('#num-req'));
    }

    if (special < requirements.Special) {
      decorateInvalid($('#special-req'));
      valid = false;
    } else {
      decorateValid($('#special-req'));
    }
  }

  return valid;
}

const errClass = 'bi-exclamation-octagon-fill text-danger';
const checkClass = 'bi-check-square-fill text-success';

function decorateInvalid($element) {
  $element.find('.bi').removeClass(checkClass).addClass(errClass);
}

function decorateValid($element) {
  $element.find('.bi').removeClass(errClass).addClass(checkClass);
}

function loginScripts(hasCurrentUser, onLoginSuccess) {
  let $loginDiv = $('#login-div');
  let $submit = $('#submit');
  let $loginForm = $('#login-form');
  let $email = $('#email');
  let $password = $('#password');

  let $pinDiv = $('#sms-code-div');
  let $pinText = $('#code-text');
  let $pinForm = $('#sms-code-form');
  let $pin = $('#sms-code');
  let $submitPin = $('#sms-code-submit');
  let $resendPin = $('#sms-code-resend');
  let $smsChange = $('#sms-change');

  let $registeredDiv = $('#registered-div');
  let factorsContainer = document.querySelector('div#factors');

  let verId = '';
  let selectedFactorIndex = 0;

  window.recaptchaVerifier = new firebase.auth.RecaptchaVerifier('recaptcha-container', {
    'size': 'invisible',
    'expired-callback': (e) => {
      window.recaptchaVerifier.reset();
    },
    'error-callback': (e) => {
      window.recaptchaVerifier.reset();
    },
  });

  $loginForm.on('submit', function (event) {
    event.preventDefault();
    onSignInSubmit();
  });

  $pinForm.on('submit', function (event) {
    event.preventDefault();

    // Disable the submit button so we only attempt once.
    $submitPin.prop('disabled', true);

    // Ask user for the SMS verification code.
    let cred = firebase.auth.PhoneAuthProvider.credential(verId, $pin.val().trim());
    let multiFactorAssertion = firebase.auth.PhoneMultiFactorGenerator.assertion(cred);

    // Complete sign-in.
    resolver
      .resolveSignIn(multiFactorAssertion)
      .then(function (userCredential) {
        onLoginSuccess();
      })
      .catch(function (err) {
        flash.clear();
        flash.error(err.message);
        window.recaptchaVerifier.reset();
        $submitPin.prop('disabled', false);
      });
  });

  $resendPin.on('click', function (event) {
    event.preventDefault();
    resendPin();
  });

  $smsChange.on('click', function (event) {
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

    signInPromise
      .then(function (userCredential) {
        onLoginSuccess();
      })
      .catch(function (error) {
        if (error.code == 'auth/multi-factor-auth-required') {
          window.recaptchaVerifier.render();
          resolver = error.resolver;

          sortFactors(resolver.hints);
          populatePinText(resolver.hints);
          populateFactors(resolver.hints);

          // If there is only one registered factor, jump to the input window
          // directly.
          if (resolver.hints.length === 1) {
            resendPin();
            $registeredDiv.addClass('d-none');
          }
        } else if (error.code == 'auth/too-many-requests') {
          flash.clear();
          flash.error(error.message);
          $submit.prop('disabled', false);
        } else {
          console.error(error);
          flash.clear();
          flash.error('Sign-in failed. Please try again.');
          $submit.prop('disabled', false);
        }
      });
  }

  function resendPin() {
    $submitPin.prop('disabled', false);
    $loginDiv.addClass('d-none');
    $pinDiv.removeClass('d-none');

    $resendPin.addClass('disabled');
    setTimeout(function () {
      $resendPin.removeClass('disabled');
    }, 15000);

    let phoneInfoOptions = {
      multiFactorHint: resolver.hints[selectedFactorIndex],
      session: resolver.session,
    };
    populatePinText(resolver.hints);
    let phoneAuthProvider = new firebase.auth.PhoneAuthProvider();
    let appVerifier = window.recaptchaVerifier;
    phoneAuthProvider
      .verifyPhoneNumber(phoneInfoOptions, appVerifier)
      .then(function (verificationId) {
        verId = verificationId;
      })
      .catch(function (error) {
        window.recaptchaVerifier.reset();
        flash.clear();
        flash.error(error.message);
        $submit.prop('disabled', false);
      });
  }

  function populatePinText(factors) {
    let factorTargetContainer = document.querySelector('#factor-target');
    let factor = factors[selectedFactorIndex];

    factorTargetContainer.textContent = `${factor.displayName} (${factor.phoneNumber})`;
  }

  function sortFactors(factors) {
    factors.sort((a, b) => {
      return a.displayName.localeCompare(b.displayName, undefined, { sensitivity: 'base' });
    });
  }

  function populateFactors(factors) {
    factors.forEach((factor, i) => appendAuthFactor(factor, i));

    $loginDiv.addClass('d-none');
    $pinDiv.addClass('d-none');
    $registeredDiv.removeClass('d-none');
  }

  function appendAuthFactor(factor, i) {
    let template = factorsContainer.querySelector('div#factor-template');
    let item = template.cloneNode(true);
    item.classList.remove('d-none');
    item.removeAttribute('id');

    let nameContainer = item.querySelector('.factor-name');
    nameContainer.textContent = factor.displayName;

    let numberContainer = item.querySelector('.factor-number');
    numberContainer.textContent = factor.phoneNumber;

    item.addEventListener('click', () => {
      $registeredDiv.addClass('d-none');
      $pinDiv.removeClass('d-none');
      selectedFactorIndex = i;
      resendPin();
    });

    factorsContainer.appendChild(item);
  }
}

// generates a random alphanumeric code
function genRandomString(len) {
  let i = len;
  let s = '';
  for (; i >= 6; i -= 6) {
    s += Math.random().toString(36).substr(2, 8);
  }
  if (i > 0) {
    s += Math.random()
      .toString(36)
      .substr(2, 2 + i);
  }
  return s;
}

// getUrlVars gets the URL params.
function getUrlVars() {
  let vars = [];
  let queryParams = window.location.href.slice(window.location.href.indexOf('?') + 1).split('&');
  for (let i = 0; i < queryParams.length; i++) {
    v = queryParams[i].split('=');
    vars.push(v[0]);
    vars[v[0]] = v[1];
  }
  return vars;
}

// element is expected to be a dom query selector, ts is the number of seconds
// since epoch, UTC.
function countdown(element, ts, expiredCallback) {
  if (typeof ts === 'undefined') {
    return;
  }

  let $element = $(element);
  let date = new Date(ts * 1000).getTime();

  const formattedTime = function () {
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
  const fn = setInterval(function () {
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

    let expiredText = $element.data('countdownExpired');
    if (!expiredText) {
      expiredText = 'EXPIRED';
    }
    return element.html(expiredText);
  }

  let prefix = $element.data('countdownPrefix');
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

function redrawCharts(chartsData, timeout) {
  let redrawPending = false;
  let windowWidth = 0;
  $(window).resize(function () {
    let w = $(window).width();
    if (w != windowWidth) {
      windowWidth = w;
    } else {
      return;
    }

    if (!redrawPending) {
      redrawPending = true;
      setTimeout(function () {
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

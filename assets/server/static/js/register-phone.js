(() => {
  window.addEventListener('load', async () => {
    const formRegister = document.querySelector('form#register-form');

    if (formRegister === null) {
      return;
    }

    const containerRegister = formRegister.querySelector('div#register-div');
    const inputPhoneNumber = formRegister.querySelector('input#phone');
    const inputDisplayName = formRegister.querySelector('input#display');
    const btnRegister = formRegister.querySelector('#submit-register');

    const formMFAPin = document.querySelector('form#sms-code-form');
    const containerMFAPin = formMFAPin.querySelector('div#sms-code-div');
    const inputMFACode = formMFAPin.querySelector('input#sms-code');
    const btnResendPin = formMFAPin.querySelector('#sms-code-resend');
    const btnChangeLoginFactor = formMFAPin.querySelector('#sms-change');
    const btnMFASubmit = formMFAPin.querySelector('button#sms-code-submit');

    const containerRegisteredFactors = document.querySelector('div#registered-div');
    const containerFactors = containerRegisteredFactors.querySelector('div#factors');

    // verificationID is the current MFA verification ID.
    let verificationID = '';

    // Initialize international telephone input formatter.
    const itiPhone = window.intlTelInput(inputPhoneNumber, {
      nationalMode: true,
      initialCountry: inputPhoneNumber.getAttribute('data-default-country'),
      utilsScript: 'https://cdnjs.cloudflare.com/ajax/libs/intl-tel-input/17.0.2/js/utils.js',
    });

    // Set up recaptcha.
    window.recaptchaVerifier = new firebase.auth.RecaptchaVerifier('recaptcha-container', {
      'size': 'invisible',
      'expired-callback': (e) => {
        window.recaptchaVerifier.reset();
      },
      'error-callback': (e) => {
        window.recaptchaVerifier.reset();
      },
    });

    window.recaptchaVerifier.render().then(function (widgetId) {
      window.recaptchaWidgetId = widgetId;
    });

    // Load the current user from firebase.
    firebase.auth().onAuthStateChanged((user) => {
      if (!user) {
        window.location.assign('/signout');
        return;
      }

      // Display any existing registered MFA devices.
      const factorsLoading = containerFactors.querySelector('div#factors-loading');
      if (user.multiFactor.enrolledFactors.length > 0) {
        factorsLoading.parentElement.removeChild(factorsLoading);

        user.multiFactor.enrolledFactors
          .sort((a, b) => a.displayName.localeCompare(b.displayName, undefined, { sensitivity: 'base' }))
          .forEach((factor) => appendAuthFactor(factor));
      } else {
        factorsLoading.querySelector('*').innerText = 'You have no registered factors.';
      }
    });

    // modifyUI alters the UI based on whether this is coming from the account
    // page or registration.
    const modifyUI = () => {
      // The "input MFA pin" div should not show "choose another factor".
      btnChangeLoginFactor.classList.add('d-none');
      btnResendPin.classList.add('d-none');
    };
    modifyUI();

    // resetUI resets the UI to its original state.
    const resetUI = () => {
      flash.clear();

      containerRegister.classList.remove('d-none');
      btnRegister.disabled = false;

      containerMFAPin.classList.add('d-none');
      btnMFASubmit.disabled = false;

      containerRegisteredFactors.classList.remove('d-none');
    };

    // appendAuthFactor builds an auth factor in the UI.
    const appendAuthFactor = (factor) => {
      let template = containerFactors.querySelector('div#factor-template');
      let item = template.cloneNode(true);
      item.id = `factor-${factor.uid}`;
      item.classList.remove('d-none');

      let containerName = item.querySelector('.factor-name');
      containerName.textContent = factor.displayName;

      let containerNumber = item.querySelector('.factor-number');
      containerNumber.textContent = factor.phoneNumber;

      let iconUnenroll = item.querySelector('.bi');
      iconUnenroll.addEventListener('click', () => unenrollFactor(factor));

      let d = new Date(factor.enrollmentTime);
      let containerEnrolled = item.querySelector('.factor-enrolled');
      containerEnrolled.textContent = `Enrolled at ${d.toLocaleString()}`;

      containerFactors.appendChild(item);
    };

    const unenrollFactor = async (factor) => {
      if (window.confirm(`Are you sure you want to unenroll ${factor.displayName} as a MFA device?`) !== true) {
        return;
      }

      const user = firebase.auth().currentUser;

      try {
        await user.multiFactor.unenroll(factor);
        await updateSession(user);
        window.location.reload();
      } catch (err) {
        if (err.code == 'auth/requires-recent-login') {
          window.location.assign('/login?redir=login/register-phone');
          return;
        }

        resetUI();
        flash.error(err.message);
      }
    };

    // updateSession updates the user attributes on the stored session.
    const updateSession = async (user) => {
      const factorCount = user.multiFactor.enrolledFactors.length;
      const idToken = await user.getIdToken();

      $.ajax({
        type: 'POST',
        url: '/session',
        data: {
          idToken: idToken,
          factorCount: factorCount,
        },
        headers: { 'X-CSRF-Token': getCSRFToken() },
        contentType: 'application/x-www-form-urlencoded',
        success: () => {
          flash.clear();
          flash.alert('Successfully updated SMS authentication.');
        },
        error: (xhr, status, err) => {
          window.location.assign('/signout');
        },
      });
    };

    // Handle submit of the register factor form.
    formRegister.addEventListener('submit', async (event) => {
      event.preventDefault();

      // Disable button to prevent duplicate submissions.
      btnRegister.disabled = true;

      // Get the current firebase session and user.
      const user = firebase.auth().currentUser;
      const mfaSession = await user.multiFactor.getSession();

      // Grab the E.164-formatted phone number.
      const phoneNumber = itiPhone.getNumber().trim();

      // Send a code and display the form.
      const authProvider = new firebase.auth.PhoneAuthProvider();

      try {
        const opts = {
          phoneNumber: phoneNumber,
          session: mfaSession,
        };
        verificationID = await authProvider.verifyPhoneNumber(opts, window.recaptchaVerifier);

        containerRegister.classList.add('d-none');
        containerMFAPin.classList.remove('d-none');
        containerRegisteredFactors.classList.add('d-none');

        inputMFACode.focus();
      } catch (err) {
        if (err.code === 'auth/requires-recent-login') {
          window.location.assign('/login?redir=login/register-phone');
          return;
        }

        resetUI();
        flash.error(err.message);
      }
    });

    formMFAPin.addEventListener('submit', async (event) => {
      event.preventDefault();

      const verificationCode = inputMFACode.value.trim();
      const displayName = inputDisplayName.value.trim();

      const credential = firebase.auth.PhoneAuthProvider.credential(verificationID, verificationCode);
      const mfaAssertion = firebase.auth.PhoneMultiFactorGenerator.assertion(credential);

      try {
        const user = firebase.auth().currentUser;
        const enrollment = user.multiFactor.enroll(mfaAssertion, displayName);
        await updateSession(user);
        window.location.reload();
      } catch (err) {
        if (err.code == 'auth/requires-recent-login') {
          window.location.assign('/login?redir=login/register-phone');
          return;
        }

        resetUI();
        flash.error(err.message);
      }
    });
  });
})();

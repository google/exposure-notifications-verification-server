(() => {
  window.addEventListener('load', () => {
    let form = document.querySelector('form#users-form');

    if (form === null) {
      return;
    }

    let permissionImpliedWarning = document.querySelector('span#permission-implied-warning');

    let inputPermissions = form.querySelectorAll('input[name=permissions]');

    inputPermissions.forEach((input) => {
      toggleState(input);

      input.addEventListener('change', (event) => {
        toggleState(event.target);
      });
    });

    function toggleState(input) {
      // Do nothing if disabled
      if (input.disabled) {
        return;
      }

      let permissionName = input.dataset.permissionName;
      let impliedPermissions = (input.dataset.impliedPermissions || '').split(',');

      let impliedPermissionsList = [];
      for (i = 0; i < impliedPermissions.length; i++) {
        let permission = impliedPermissions[i].trim();
        if (permission) {
          impliedPermissionsList.push(permission);
        }
      }

      let inputs = impliedPermissionsList
        .map(function (permission) {
          return `input[data-permission-name="${permission}"]`;
        })
        .join(',');
      let labels = impliedPermissionsList
        .map(function (permission) {
          return `label#permission-${permission}-label`;
        })
        .join(',');

      if (input.checked) {
        if (inputs) {
          form.querySelectorAll(inputs).forEach((implied) => {
            implied.checked = true;
            implied.disabled = true;
          });
        }

        let warning = permissionImpliedWarning.cloneNode(true);
        warning.classList.remove('d-none');
        warning.setAttribute('title', `Implied by ${permissionName}`);

        // TODO(sethvargo): drop jquery dependency and switch from load to
        // DOMContentLoaded
        $(warning).tooltip();

        if (labels) {
          form.querySelectorAll(labels).forEach((label) => {
            label.querySelectorAll('small.form-text').forEach((helpText) => {
              helpText.insertAdjacentElement('beforeBegin', warning);
            });
          });
        }
      } else {
        if (inputs) {
          form.querySelectorAll(inputs).forEach((implied) => {
            implied.disabled = false;
          });
        }

        if (labels) {
          form.querySelectorAll(labels).forEach((label) => {
            label.querySelectorAll('span.oi').forEach((icon) => {
              icon.parentNode.removeChild(icon);
            });
          });
        }
      }
    }
  });
})();

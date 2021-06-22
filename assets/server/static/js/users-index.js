(() => {
  window.addEventListener('DOMContentLoaded', () => {
    if (document.querySelector('body#users-index') === null) {
      return;
    }

    let form = document.querySelector('form#users-form');
    let userCheckboxes = form.querySelectorAll('input[name=user_id]');

    let inputSelectUserAll = form.querySelector('input#select-user-all');

    let btnAddPermissions = form.querySelector('button#add-permissions-button');
    let btnAddPermissionsSubmit = form.querySelector('input#add-permissions-submit');

    let btnRemovePermissions = form.querySelector('button#remove-permissions-button');
    let btnRemovePermissionsSubmit = form.querySelector('input#remove-permissions-submit');

    const updateUI = () => {
      let checked = 0;
      userCheckboxes.forEach((checkbox) => {
        if (checkbox.checked) {
          checked++;
        }
      });

      if (checked > 0) {
        let users = 'users';
        if (checked === 1) {
          users = 'user';
        }

        btnAddPermissions.disabled = false;
        btnAddPermissionsSubmit.value = `Add permissions to ${checked} ${users}`;
        btnRemovePermissions.disabled = false;
        btnRemovePermissionsSubmit.value = `Remove permissions from ${checked} ${users}`;
      } else {
        btnAddPermissions.disabled = true;
        btnRemovePermissions.disabled = true;
      }
    };

    userCheckboxes.forEach((checkbox) => {
      checkbox.addEventListener('change', updateUI);
    });

    inputSelectUserAll.addEventListener('change', (event) => {
      let checked = event.target.checked;
      userCheckboxes.forEach((item) => {
        item.checked = checked;
      });
      updateUI();
    });

    updateUI();
  });
})();

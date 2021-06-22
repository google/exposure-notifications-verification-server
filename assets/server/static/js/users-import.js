(() => {
  // batchSize is the number of individual requests to bundle into a single
  // upstream API request.
  const batchSize = 10;

  window.addEventListener('DOMContentLoaded', () => {
    if (document.querySelector('body#users-import') === null) {
      return;
    }

    let $form = $('#form');
    let $csv = $('#csv');
    let $fileLabel = $('#fileLabel');
    let $import = $('#import');
    let $cancel = $('#cancel');
    let $table = $('#csv-table');
    let $tableBody = $('#csv-table-body');
    let $progressDiv = $('#progress-div');
    let $progress = $('#progress');
    let $sendInvites = $('#sendInvites');

    let totalUsersAdded = 0;
    let upload = readFile();

    $table.hide();

    if (typeof FileReader == 'undefined') {
      flash.error('Your browser does not support the required HTML5 file reader.');
    } else {
      $csv.prop('disabled', false);
    }

    $csv.change(function (file) {
      let fileName = file.target.files[0].name;
      $fileLabel.html(fileName);
      $import.prop('disabled', false);
    });

    $cancel.on('click', function (event) {
      upload.cancel();
      flash.error('Canceled batch upload.');
    });

    $form.on('submit', function (event) {
      event.preventDefault();
      $import.prop('disabled', true);
      $cancel.prop('disabled', false);

      $table.show(100);
      $progressDiv.removeClass('d-none');

      let reader = new FileReader();
      reader.onload = upload.start;
      reader.readAsText($csv[0].files[0]);
    });

    function readFile() {
      // State for managing cleanup and canceling
      let cancelUpload = false;
      let cancel = () => {
        cancelUpload = true;
      };

      let start = async function (e) {
        let checked = $sendInvites.is(':checked');
        let rows = e.target.result.split('\n');
        let batch = [];
        totalUsersAdded = 0;
        $tableBody.empty();
        let i = 0;
        for (; i < rows.length && !cancelUpload; i++) {
          // Clear batch that was just uploaded.
          if (batch.length >= batchSize) {
            $tableBody.empty();
            batch = [];
          }

          // Add to batch if the next row is valid.
          if (rows[i].trim() != '') {
            let user = {};
            let cols = rows[i].split(',');
            user['email'] = cols[0].trim();
            user['name'] = cols.length > 1 ? cols[1].trim() : '';

            let row = '<tr><td>' + user['email'] + '</td><td>' + user['name'] + '</td></tr>';
            $tableBody.append(row);

            batch.push(user);
          }

          // If we've hit the batch limit or end of file, upload it.
          if (batch.length >= batchSize || (i == rows.length - 1 && batch.length > 0)) {
            cancelUpload = await uploadWithRetries(() => uploadBatch(batch, checked));
            if (cancelUpload) {
              flash.warning(
                'Successfully added ' + totalUsersAdded + ' users to realm.' + (rows.length - i) + ' remaining.'
              );
              break;
            }

            let percent = Math.floor(((i + 1) * 100) / rows.length) + '%';
            $progress.width(percent);
            $progress.html(percent);
          }
        }

        if (!cancelUpload) {
          flash.alert('Successfully added ' + totalUsersAdded + ' users to realm.');
        }
        $table.fadeOut(400);
        $import.prop('disabled', false);
        $cancel.prop('disabled', true);
      };

      return { start, cancel };
    }
  });

  function uploadBatch(data, sendInvites) {
    return $.ajax({
      type: 'POST',
      url: '/realm/users/import',
      data: JSON.stringify({
        users: data,
        sendInvites: sendInvites,
      }),
      headers: { 'X-CSRF-Token': getCSRFToken() },
      contentType: 'application/json',
      success: function (result) {
        totalUsersAdded += result.newUsers.length;
        if (result.error) {
          flash.error(result.error);
        }
      },
      error: function (xhr, status, e) {
        flash.error(e);
      },
    });
  }
})();

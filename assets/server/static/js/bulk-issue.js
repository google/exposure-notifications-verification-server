(() => {
  // batchSize is the number of individual requests to bundle into a single
  // upstream API request. This number is defined in handle_issue_batch.go.
  const batchSize = 10;

  // showMaxResults is the maximum number of entries to display in the HTML
  // table embedded on the page. If more than showMaxResults exist, their status
  // will be in the CSV result.
  const showMaxResults = 50;

  // retryCodeCookieName is the name of cookie where the retry code is saved.
  const retryCodeCookieName = 'retryCode';

  // Attempt to load the bulk-issue UI and uploader if the components exist.
  window.addEventListener('DOMContentLoaded', () => {
    if (document.querySelector('body#bulk-issue') === null) {
      return;
    }

    let total = 0;
    let totalErrs = 0;

    let now = new Date();
    let tzOffset = now.getTimezoneOffset();

    let $form = $('#form');
    let $csv = $('#csv');
    let $import = $('#import');
    let $cancel = $('#cancel');
    let $table = $('#csv-table');
    let $tableBody = $('#csv-table-body');
    let $progressDiv = $('#progress-div');
    let $progress = $('#progress');
    let $retryCode = $('#retry-code');
    let $rememberCode = $('#remember-code');
    let $inputSMSTemplate = $('select#sms-template');
    let $newCode = $('#new-code');
    let $startAt = $('#start-at');

    let $receiptDiv = $('#receipt-div');
    let $save = $('#save');
    let $receiptSuccess = $('#receipt-success');
    let $receiptFailure = $('#receipt-failure');

    let $errorDiv = $('#error-div');
    let $errorTable = $('#error-table');
    let $errorTableBody = $('#error-table > tbody');
    let $errorTooMany = $('#error-too-many');

    let $successDiv = $('#success-div');
    let $successTable = $('#success-table');
    let $successTableBody = $('#success-table > tbody');
    let $successTooMany = $('#success-too-many');

    $save.attr('download', `${now.toISOString().split('T')[0]}-bulk-issue-log.csv`);

    let randomString = getCookie(retryCodeCookieName);
    if (randomString == '') {
      randomString = genRandomString(12);
    } else {
      $rememberCode.prop('checked', true);
    }
    $retryCode.val(randomString);
    let upload = readBulkUploadCSVFile();

    if (typeof FileReader === 'undefined') {
      flash.error('Please update to a browser which supports the HTML5 file reader API.');
    }

    $csv.change(function (file) {
      $startAt.val(1);
    });

    $cancel.on('click', function (event) {
      event.preventDefault();
      upload.cancel();
      flash.error('Canceled batch upload.');
    });

    $newCode.on('click', function (event) {
      event.preventDefault();
      $retryCode.val(genRandomString(12));
    });

    $form.on('submit', function (event) {
      event.preventDefault();
      resetBulkUploadUI();

      if ($rememberCode.is(':checked')) {
        setCookie(retryCodeCookieName, $retryCode.val(), 1);
      } else {
        setCookie(retryCodeCookieName, '', -1);
      }

      let reader = new FileReader();
      reader.onload = upload.start;
      reader.readAsText($csv[0].files[0]);
    });

    function resetBulkUploadUI() {
      $cancel.removeClass('d-none');

      $table.removeClass('d-none');
      $progressDiv.removeClass('d-none');

      $receiptDiv.addClass('d-none');
      $save.attr('href', 'data:text/plain,');
      $receiptSuccess.text(0);
      $receiptFailure.text(0);

      $errorTooMany.addClass('d-none');
      $errorDiv.addClass('d-none');
      $errorTableBody.empty();

      $successTooMany.addClass('d-none');
      $successDiv.addClass('d-none');
      $successTableBody.empty();
    }

    function readBulkUploadCSVFile() {
      // State for managing cleanup and canceling
      let cancelUpload = false;
      let cancel = () => {
        cancelUpload = true;
      };

      let start = async function (e) {
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
          if (request != '') {
            batch.push(request);
            batchLines.push(i + 1);
          }

          // If we've hit the batch limit or end of file, upload it.
          if (batch.length >= batchSize || (i == rows.length - 1 && batch.length > 0)) {
            $tableBody.empty();
            for (let r = 0; r < batch.length; r++) {
              let $row = $('<tr/>');
              $row.append($('<td/>').text(batch[r]['phone']));
              $row.append($('<td/>').text(batch[r]['testDate']));
              $tableBody.append($row);
            }

            cancelUpload = await uploadWithRetries(() => uploadBatchIssue(batch, batchLines));

            if (cancelUpload) {
              if (total > 0) {
                flash.warning(`Successfully issued ${total} codes. ${rows.length - i} +  remaining.`);
              }
              break;
            }
            $startAt.val(i + 1);
            let percent = Math.floor(((i + 1) * 100) / rows.length) + '%';
            $progress.width(percent);
            $progress.html(percent);
          }
        }

        $save.attr('href', $save.attr('href') + '\n');

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

        $cancel.addClass('d-none');
        $table.addClass('d-none');
        $tableBody.empty();
      };

      return { start, cancel };
    }

    function buildBatchIssueRequest(thisRow, retryCode, template, line) {
      thisRow = thisRow.trim();
      if (thisRow == '') {
        return '';
      }
      let request = {};
      let cols = thisRow.split(',');

      // Escape csv row contents
      request['phone'] = $('<div>').text(cols[0].trim()).html();
      request['testDate'] = cols.length > 1 ? $('<div>').text(cols[1].trim()).html() : '';
      request['symptomDate'] = cols.length > 2 ? $('<div>').text(cols[2].trim()).html() : '';
      request['testType'] = cols.length > 3 ? $('<div>').text(cols[3].trim()).html() : 'confirmed';

      if (request['testType'] == '') {
        request['testType'] = 'confirmed';
      }

      // Skip missing phone number
      if (request['phone'] == '' || cols.Length < 2) {
        let code = {
          errorCode: 'invalid_client',
          error: 'phone number missing',
        };
        showErroredCode(request, code, line);
        return '';
      }

      let uuid = '';
      if (cols.length > 6) {
        uuid = $('<div>').text(cols[6].trim()).html();
      }
      if (uuid.length != 36) {
        // Generate a UUID by hashing phone
        let hs = String(CryptoJS.HmacSHA256(request['phone'], retryCode)).substr(0, 36);
        uuid =
          hs.substr(0, 8) +
          '-' +
          hs.substr(9, 4) +
          '-' +
          hs.substr(13, 4) +
          '-' +
          hs.substr(17, 4) +
          '-' +
          hs.substr(21, 12);
      }

      request['uuid'] = uuid;
      request['smsTemplateLabel'] = template;
      request['tzOffset'] = tzOffset;

      // CSV file has error codes in the file. Usually means a retry of the receipt file.
      // Skip un-retryable errors
      if (cols.length >= 8) {
        let errCode = $('<div>').text(cols[7].trim()).html();
        if (errCode == 'success') {
          let code = {
            errorCode: errCode,
            error: `code uuid ${uuid} already succeeded. skipping code.`,
          };
          showErroredCode(request, code, line);
          return '';
        } else if (errCode == 'uuid_already_exists') {
          let existingUUID = $('<div>').text(cols[6].trim()).html();
          if (uuid == existingUUID) {
            let code = {
              errorCode: errCode,
              error: `code uuid ${existingUUID} already exists on the server. skipping code.`,
            };
            showErroredCode(request, code, line);
            return '';
          }
        }
      }
      return request;
    }

    function uploadBatchIssue(data, lines) {
      let req = {
        codes: data,
        // Request is padded with 5-15 random chars. These are ignored but vary the size of the request
        // to prevent network traffic observation.
        padding: btoa(genRandomString(5 + Math.floor(Math.random() * 15))),
      };
      return $.ajax({
        url: '/codes/batch-issue',
        type: 'POST',
        dataType: 'json',
        cache: false,
        contentType: 'application/json',
        headers: { 'X-CSRF-Token': getCSRFToken() },
        data: JSON.stringify(req),
        success: function (result) {
          if (!result || !result.codes) {
            return;
          }
          readCodesBatch(data, lines, result.codes);
        },
        error: function (xhr, resp, text) {
          if (!xhr || !xhr.responseJSON) {
            return;
          }

          if (!xhr.responseJSON.codes) {
            let message = resp;
            if (xhr.responseJSON.error) {
              message = message + ': ' + xhr.responseJSON.error;
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
      if (code.errorCode != 'success') {
        totalErrs++;
      }
      if (totalErrs > 0) {
        $receiptDiv.removeClass('d-none');
        $errorDiv.removeClass('d-none');
      }
      if (totalErrs == showMaxResults + 1) {
        $errorTableBody.empty();
        $errorTable.addClass('d-none');
        $errorTooMany.removeClass('d-none');
      }
      $receiptFailure.text(totalErrs);
      $save.attr(
        'href',
        `${$save.attr('href')}${request['phone']},${request['testDate']},${request['symptomDate']},${
          request['testType']
        },,,${request['uuid']},${code.errorCode},${code.error}\n`
      );
      if (totalErrs > showMaxResults) {
        return;
      }

      let $row = $('<tr/>');
      $row.append($('<td/>').text(line));
      $row.append($('<td/>').text(request['phone']));
      $row.append($('<td/>').text(request['testDate']));
      $row.append($('<td/>').text(code.error));
      $errorTableBody.append($row);
    }

    function showSuccessfulCode(request, code, line) {
      total++;
      if (total > 0) {
        $receiptDiv.removeClass('d-none');
        $successDiv.removeClass('d-none');
        $successTable.removeClass('d-none');
      }
      if (total == showMaxResults + 1) {
        $successTableBody.empty();
        $successTable.addClass('d-none');
        $successTooMany.removeClass('d-none');
      }
      $receiptSuccess.text(total);
      $save.attr(
        'href',
        `${$save.attr('href')}${request['phone']},${request['testDate']},${request['symptomDate']},${
          request['testType']
        },,,${code.uuid},success\n`
      );
      if (total > showMaxResults) {
        return;
      }

      let $row = $('<tr/>');
      $row.append($('<td/>').text(line));
      $row.append($('<td/>').text(request['phone']));
      $row.append($('<td/>').text(request['testDate']));
      $row.append($('<td/>').text(code.uuid));
      $successTableBody.append($row);
    }
  });
})();

(() => {
  window.addEventListener('load', (event) => {
    const container = document.querySelector('div#per_external_issuer_table');
    if (!container) {
      return;
    }

    const template = document.querySelector('div#external_issuer_row_template');
    if (!template) {
      return;
    }
    const templateListGroup = template.querySelector('.list-group');
    const [templateRow, templateTable] = template.querySelectorAll('.list-group-item');

    const request = new XMLHttpRequest();
    request.open('GET', '/stats/realm/external-issuers.json');
    request.overrideMimeType('application/json');

    request.onload = (event) => {
      const pContainer = container.querySelector('p');

      const data = JSON.parse(request.response);
      if (!data.statistics) {
        pContainer.innerText = 'There is no external issuer data yet.';
        return;
      }

      const listGroup = templateListGroup.cloneNode(true);
      for (let i = 0; i < data.statistics.length; i++) {
        const stat = data.statistics[i];
        const date = utcDate(stat.date);
        const id = `collapse-external-${date.getTime()}`;

        const item = templateRow.cloneNode(true);
        item.classList.remove('d-none');
        item.setAttribute('data-bs-target', `#${id}`);
        item.setAttribute('aria-controls', `${id}`);
        item.innerText = date.toLocaleDateString();
        listGroup.appendChild(item);

        const tableDiv = templateTable.cloneNode(true);
        tableDiv.id = id;
        listGroup.appendChild(tableDiv);

        const tbody = tableDiv.querySelector('table > tbody');
        for (let j = 0; j < stat.issuer_data.length; j++) {
          const issuerData = stat.issuer_data[j];

          const tr = document.createElement('tr');
          tbody.appendChild(tr);

          const tdID = document.createElement('td');
          tdID.innerText = issuerData.issuer_id;
          tr.appendChild(tdID);

          const tdIssued = document.createElement('td');
          tdIssued.innerText = issuerData.codes_issued;
          tr.appendChild(tdIssued);
        }
      }

      clearChildren(container);
      container.appendChild(listGroup);
    };

    request.onerror = (event) => {
      console.error('error from response: ' + request.response);
      flash.error('Failed to render external issuer stats: ' + err);
    };

    request.send();
  });
})();

(() => {
  window.addEventListener('load', async (event) => {
    const containerChart = document.querySelector('div#keyserver_chart_div');
    if (!containerChart) {
      return;
    }

    const tekSlider = document.querySelector('#tek_slider');
    const onsetSlider = document.querySelector('#onset_slider');
    const smoothDrop = document.querySelector('#smooth-drop');
    addEventListeners(smoothDrop, 'change', async (event) => {
      const elem = event.target;
      const smoothing = elem.value;

      tekSlider.setAttribute('min', Math.min(smoothing, tekSlider.getAttribute('max')));
      tekSlider.value = tekSlider.getAttribute('max');
      tekSlider.dispatchEvent(new Event('input'));

      onsetSlider.setAttribute('min', Math.min(smoothing, onsetSlider.getAttribute('max')));
      onsetSlider.value = onsetSlider.getAttribute('max');
      onsetSlider.dispatchEvent(new Event('input'));
    });

    google.charts.load('current', {
      packages: ['corechart', 'controls'],
      callback: drawCharts,
    });

    function drawCharts() {
      const request = new XMLHttpRequest();
      request.open('GET', '/stats/realm/key-server.json');
      request.overrideMimeType('application/json');

      request.onload = (event) => {
        const data = JSON.parse(request.response);
        drawTotalTEKsPublishedChart(data);
        drawOSChart(data);
        drawDaysActiveBeforeUploadChart(data);
        drawOnsetToUploadChart(data);
      };

      request.onerror = (event) => {
        console.error('error from response: ' + request.response);
        flash.error('Failed to load key server stats: ' + err);
      };

      request.send();
    }

    // drawTotalTEKsPublishedChart draws the statistics for the Total TEKs
    // Published chart.
    function drawTotalTEKsPublishedChart(data) {
      const dashboardContainer = document.querySelector('div#keyserver_dashboard_div');
      const chartContainer = dashboardContainer.querySelector('div#keyserver_chart_div');
      const filterContainer = dashboardContainer.querySelector('div#keyserver_filter_div');

      if (!data) {
        const pContainer = chartContainer.querySelector('p');
        pContainer.innerText = 'No data yet.';
        return;
      }

      const dataTable = new google.visualization.DataTable();
      dataTable.addColumn('date', 'Date');
      dataTable.addColumn('number', 'Total # TEKs published');

      for (let i = 0; i < data.length; i++) {
        const stat = data[i];
        dataTable.addRow([utcDate(stat.day), stat.total_teks_published]);
      }

      const win = Math.min(30, data.length - 1);
      const startChart = new Date(data[win].day);

      const dateFormatter = new google.visualization.DateFormat({
        pattern: 'MMM dd',
      });
      dateFormatter.format(dataTable, 0);

      const dashboard = new google.visualization.Dashboard(dashboardContainer);

      const filter = new google.visualization.ControlWrapper({
        controlType: 'ChartRangeFilter',
        containerId: filterContainer,
        state: {
          range: {
            start: startChart,
          },
        },
        options: {
          filterColumnIndex: 0,
          series: {
            0: {
              opacity: 0,
            },
          },
          ui: {
            chartType: 'LineChart',
            chartOptions: {
              colors: ['#dddddd'],
              chartArea: {
                width: '100%',
                height: '100%',
                top: 0,
                right: 40,
                bottom: 20,
                left: 60,
              },
              isStacked: true,
              hAxis: { format: 'M/d' },
            },
            chartView: {
              columns: [0, 1],
            },
            minRangeSize: 86400000, // ms for 1 day
          },
        },
      });

      const chart = new google.visualization.ChartWrapper({
        chartType: 'LineChart',
        containerId: chartContainer,
        options: {
          colors: ['#28a745'],
          chartArea: {
            left: 60,
            right: 40,
            bottom: 40,
            top: 40,
            width: '100%',
            height: '300',
          },
          hAxis: { format: 'M/d' },
          legend: { position: 'top' },
          width: '100%',
        },
      });

      dashboard.bind(filter, chart);
      dashboard.draw(dataTable);
      debounce('resize', async () => dashboard.draw(dataTable));
    }

    // drawOSChart draws a chart showing uploads by operating system.
    function drawOSChart(data) {
      const dashboardContainer = document.querySelector('div#keyserver_os_dashboard_div');
      const chartContainer = dashboardContainer.querySelector('div#keyserver_os_chart_div');
      const filterContainer = dashboardContainer.querySelector('div#keyserver_os_filter_div');

      const allowsUserReport = chartContainer.dataset.realmAllowsUserReport === 'true';

      if (!data) {
        const pContainer = chartContainer.querySelector('p');
        pContainer.innerText = 'No data yet.';
        return;
      }

      const win = Math.min(30, data.length - 1);
      const startChart = new Date(data[win].day);

      const dataTable = new google.visualization.DataTable();
      dataTable.addColumn('date', 'Date');
      dataTable.addColumn('number', 'Total');
      dataTable.addColumn({ type: 'string', role: 'tooltip', p: { html: true } });
      dataTable.addColumn('number', 'Missing Onset Date');
      if (allowsUserReport) {
        dataTable.addColumn('number', 'Key Revision Requests');
      }
      dataTable.addColumn('number', 'Unknown OS');
      dataTable.addColumn({ type: 'string', role: 'tooltip', p: { html: true } });
      dataTable.addColumn('number', 'Android');
      dataTable.addColumn({ type: 'string', role: 'tooltip', p: { html: true } });
      dataTable.addColumn('number', 'IOS');
      dataTable.addColumn({ type: 'string', role: 'tooltip', p: { html: true } });

      for (let i = 0; i < data.length; i++) {
        const stat = data[i];
        const total = stat.publish_requests.unknown + stat.publish_requests.android + stat.publish_requests.ios;
        const unknownCount = parseInt(stat.publish_requests.unknown);
        const androidCount = parseInt(stat.publish_requests.android);
        const iosCount = parseInt(stat.publish_requests.ios);

        const row = [
          utcDate(stat.day),
          total,
          `
          <ul class="list-unstyled text-nowrap m-0 p-3">
            <li><strong>Total:</strong> ${total}</li>
            <li><strong>Android</strong>: ${androidCount}</li>
            <li><strong>iOS</strong>: ${iosCount}</li>
            <li><strong>Unknown</strong>: ${unknownCount}</li>
          </ul>
          `,
          stat.requests_missing_onset_date,
        ];
        if (allowsUserReport) {
          row.push(stat.requests_with_revisions);
        }
        row.push(
          stat.publish_requests.unknown,
          `
          <ul class="list-unstyled text-nowrap m-0 p-3">
            <li><strong>Unknown: </strong> ${unknownCount} (${((unknownCount / total) * 100).toPrecision(3)}%)</li>
          </ul>
          `,
          stat.publish_requests.android,
          `
          <ul class="list-unstyled text-nowrap m-0 p-3">
            <li><strong>Android: </strong> ${androidCount} (${((androidCount / total) * 100).toPrecision(3)}%)</li>
          </ul>
          `,
          stat.publish_requests.ios,
          `
          <ul class="list-unstyled text-nowrap m-0 p-3">
            <li><strong>iOS: </strong> ${iosCount} (${((iosCount / total) * 100).toPrecision(3)}%)</li>
          </ul>
          `
        );
        dataTable.addRow(row);
      }

      const dateFormatter = new google.visualization.DateFormat({
        pattern: 'MMM dd',
      });
      dateFormatter.format(dataTable, 0);

      const dashboard = new google.visualization.Dashboard(dashboardContainer);

      const filter = new google.visualization.ControlWrapper({
        controlType: 'ChartRangeFilter',
        containerId: filterContainer,
        state: {
          range: {
            start: startChart,
          },
        },
        options: {
          filterColumnIndex: 0,
          series: {
            0: {
              opacity: 0,
            },
          },
          ui: {
            chartType: 'LineChart',
            chartOptions: {
              colors: ['#dddddd'],
              chartArea: {
                width: '100%',
                height: '100%',
                top: 0,
                right: 40,
                bottom: 20,
                left: 60,
              },
              isStacked: true,
              hAxis: { format: 'M/d' },
            },
            chartView: {
              columns: [0, 1],
            },
            minRangeSize: 86400000, // ms for 1 day
          },
        },
      });

      const colors = ['#ee8c00', '#ea4335'];
      if (allowsUserReport) {
        colors.push('#fcfc3d');
      }
      colors.push('#6c757d', '#28a745', '#007bff');
      const series = {
        0: { type: 'line' },
        1: { type: 'line' },
      };
      if (allowsUserReport) {
        series['2'] = { type: 'line' };
      }

      const chart = new google.visualization.ChartWrapper({
        chartType: 'ComboChart',
        containerId: chartContainer,
        options: {
          colors: colors,
          chartArea: {
            left: 60,
            right: 40,
            bottom: 40,
            top: 40,
            width: '100%',
            height: '300',
          },
          hAxis: {
            format: 'M/d',
            gridlines: { color: 'transparent' },
          },
          seriesType: 'bars',
          series: series,
          legend: { position: 'top' },
          isStacked: true,
          tooltip: { isHtml: true },
        },
      });

      dashboard.bind(filter, chart);
      dashboard.draw(dataTable);
      debounce('resize', async () => dashboard.draw(dataTable));
    }

    function drawDaysActiveBeforeUploadChart(data) {
      const chartContainer = document.querySelector('#keyserver_tek_chart_div');

      if (!data) {
        const pContainer = chartContainer.querySelector('p');
        pContainer.innerText = 'No data yet.';
        return;
      }

      tekSlider.setAttribute('min', Math.min(smoothDrop.value, data.length));
      tekSlider.setAttribute('max', data.length);
      tekSlider.value = data.length;

      let vAxisMax = 1;
      for (let i = 0; i < data.length; i++) {
        const dataTable = getTEKDataTable(data, i);
        for (let j = 0; j < dataTable.getNumberOfColumns(); j++) {
          const result = dataTable.getColumnRange(j);
          if (result && result.max && result.max > vAxisMax) {
            vAxisMax = result.max;
          }
        }
      }

      const tekOptions = {
        colors: ['#329262'],
        chartArea: {
          left: 60,
          right: 40,
          bottom: 40,
          top: 40,
          width: '100%',
          height: '300',
        },
        legend: { position: 'none' },
        hAxis: {
          title: 'TEK days old',
          gridlines: { color: 'transparent' },
          baseline: { color: 'transparent' },
          ticks: [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14],
          showTextEvery: 1,
        },
        vAxis: {
          minValue: 0,
          maxValue: vAxisMax,
        },
        titlePosition: 'out',
        tooltip: { isHtml: true },
      };

      const tekChart = new google.visualization.ColumnChart(chartContainer);

      const drawChart = async () => {
        const idx = data.length - tekSlider.value;
        if (idx < 0) {
          idx = 0;
        }

        const date = utcDate(data[idx].day);
        tekOptions.title = `${smoothDrop.value} days from ${date.toLocaleDateString()}`;
        const tekData = getTEKDataTable(data, idx);
        tekChart.draw(tekData, tekOptions);
      };

      debounce('resize', drawChart);
      addEventListeners(tekSlider, 'input', drawChart);
      drawChart();
    }

    const tekDataTableCache = new Map();
    function getTEKDataTable(data, idx) {
      const key = `s${smoothDrop.value}i${idx}`;
      if (tekDataTableCache.has(key)) {
        return tekDataTableCache.get(key);
      }

      const dataTable = new google.visualization.DataTable();
      dataTable.addColumn('number', 'days');
      dataTable.addColumn('number', 'count');
      dataTable.addColumn({ type: 'string', role: 'style' });
      dataTable.addColumn({ type: 'string', role: 'annotation' });
      dataTable.addColumn({ type: 'string', role: 'tooltip', p: { html: true } });

      // sum over last ${smoothDrop.value} days
      const table = new Array(data[idx].tek_age_distribution.length).fill(0);
      let total = 0;
      for (let offset = 0; offset < smoothDrop.value; offset++) {
        if (idx + offset >= data.length) {
          break;
        }
        const row = data[idx + offset];
        for (let j = 0; j < row.tek_age_distribution.length; j++) {
          const count = parseInt(row.tek_age_distribution[j]);
          table[j] += count;
          total += count;
        }
      }

      for (let i = 0; i < table.length; i++) {
        const r = [i, table[i], '', '', ''];
        if (i == 15) {
          r[2] = '#6c757d';
          r[3] = '';
          r[4] = `
            <ul class="list-unstyled text-nowrap m-0 p-3">
              <li><strong>${r[0]}+ days:</strong> ${r[1]} (${((r[1] / total) * 100).toPrecision(3)}%)</li>
            </ul>
          `;
        } else {
          r[4] = `
            <ul class="list-unstyled text-nowrap m-0 p-3">
              <li><strong>${r[0]}-${r[0] + 1} days:</strong> ${r[1]} (${((r[1] / total) * 100).toPrecision(3)}%)</li>
            </ul>
          `;
        }

        dataTable.addRow(r);
      }
      tekDataTableCache.set(key, dataTable);
      return dataTable;
    }

    function drawOnsetToUploadChart(data) {
      const chartContainer = document.querySelector('#keyserver_onset_chart_div');

      if (!data) {
        const pContainer = chartContainer.querySelector('p');
        pContainer.innerText = 'No data yet.';
        return;
      }

      onsetSlider.setAttribute('min', Math.min(smoothDrop.value, data.length));
      onsetSlider.setAttribute('max', data.length);
      onsetSlider.value = data.length;

      let vAxisMax = 1;
      for (let i = 0; i < data.length; i++) {
        const dataTable = getOnsetDataTable(data, i);
        for (let j = 0; j < dataTable.getNumberOfColumns(); j++) {
          const result = dataTable.getColumnRange(j);
          if (result && result.max && result.max > vAxisMax) {
            vAxisMax = result.max;
          }
        }
      }

      const onsetOptions = {
        colors: ['#316395'],
        chartArea: {
          left: 60,
          right: 40,
          bottom: 40,
          top: 40,
          width: '100%',
          height: '300',
        },
        legend: { position: 'none' },
        hAxis: {
          title: 'Days from onset to upload',
          gridlines: { color: 'transparent' },
          baseline: { color: 'transparent' },
          ticks: [
            0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28,
            29,
          ],
          showTextEvery: 1,
        },
        vAxis: {
          minValue: 0,
          maxValue: vAxisMax,
        },
        titlePosition: 'out',
        tooltip: { isHtml: true },
      };

      const onsetChart = new google.visualization.ColumnChart(chartContainer);

      const drawChart = async () => {
        const idx = data.length - onsetSlider.value;
        if (idx < 0) {
          idx = 0;
        }

        const date = utcDate(data[idx].day);
        onsetOptions.title = `${smoothDrop.value} days from ${date.toLocaleDateString()}`;
        const onsetData = getOnsetDataTable(data, idx);
        onsetChart.draw(onsetData, onsetOptions);
      };

      debounce('resize', drawChart);
      addEventListeners(onsetSlider, 'input', drawChart);
      drawChart();
    }

    const onsetDataTableCache = new Map();
    function getOnsetDataTable(data, idx) {
      const key = `s${smoothDrop.value}i${idx}`;
      if (onsetDataTableCache.has(key)) {
        return onsetDataTableCache.get(key);
      }

      const dataTable = new google.visualization.DataTable();
      dataTable.addColumn('number', 'days');
      dataTable.addColumn('number', 'count');
      dataTable.addColumn({ type: 'string', role: 'style' });
      dataTable.addColumn({ type: 'string', role: 'annotation' });
      dataTable.addColumn({ type: 'string', role: 'tooltip', p: { html: true } });

      // sum over last ${smoothDrop.value} days
      const table = new Array(data[idx].onset_to_upload_distribution.length).fill(0);
      let total = 0;
      for (let offset = 0; offset < smoothDrop.value; offset++) {
        if (idx + offset >= data.length) {
          break;
        }
        const row = data[idx + offset];
        for (let j = 0; j < row.onset_to_upload_distribution.length; j++) {
          const count = parseInt(row.onset_to_upload_distribution[j]);
          table[j] += count;
          total += count;
        }
      }

      for (let i = 0; i < table.length; i++) {
        const r = [i, table[i], '', '', ''];
        if (i == 30) {
          r[2] = '#6c757d';
          r[3] = '';
          r[4] = `
            <ul class="list-unstyled text-nowrap m-0 p-3">
              <li><strong>${r[0]}+ days:</strong> ${r[1]} (${((r[1] / total) * 100).toPrecision(3)}%)</li>
            </ul>
          `;
        } else {
          r[4] = `
            <ul class="list-unstyled text-nowrap m-0 p-3">
              <li><strong>${r[0]}-${r[0] + 1} days:</strong> ${r[1]} (${((r[1] / total) * 100).toPrecision(3)}%)</li>
            </ul>
           `;
        }

        dataTable.addRow(r);
      }
      onsetDataTableCache.set(key, dataTable);
      return dataTable;
    }
  });
})();

(() => {
  window.addEventListener('load', async (event) => {
    const containerChart = document.querySelector('div#realm_chart_div');
    if (!containerChart) {
      return;
    }

    const issueAgeSlider = document.querySelector('#issue_age_slider');
    const smoothDrop = document.querySelector('#smooth-drop');
    addEventListeners(smoothDrop, 'change', async (event) => {
      const elem = event.target;
      const smoothing = elem.value;

      issueAgeSlider.setAttribute('min', Math.min(smoothing, issueAgeSlider.getAttribute('max')));
      issueAgeSlider.value = issueAgeSlider.getAttribute('max');
      issueAgeSlider.dispatchEvent(new Event('input'));
    });

    google.charts.load('current', {
      packages: ['corechart', 'controls'],
      callback: drawCharts,
    });

    function drawCharts() {
      const request = new XMLHttpRequest();
      request.open('GET', '/stats/realm/composite.json');
      request.overrideMimeType('application/json');

      request.onload = (event) => {
        const data = JSON.parse(request.response);
        drawClaimChart(data);
        drawMeanClaimAgeChart(data);
        drawClaimAgeChart(data);
      };

      request.onerror = (event) => {
        console.error('error from response: ' + request.response);
        flash.error('Failed to load realm stats: ' + err);
      };

      request.send();
    }

    function drawClaimChart(data) {
      const charts = [
        {
          chartType: 'LineChart',
          chartDiv: '#realm_chart_div',
          dashboardDiv: '#dashboard_div',
          filterDiv: '#filter_div',
          headerFunc: (dataTable, hasKeyServerStats) => {
            dataTable.addColumn('date', 'Date');
            dataTable.addColumn('number', 'Codes Issued');
            dataTable.addColumn('number', 'Codes Claimed');
            dataTable.addColumn('number', 'Invalid Codes');
            dataTable.addColumn('number', 'Tokens Claimed');
            if (hasKeyServerStats) {
              dataTable.addColumn('number', 'Publish Requests');
            }
          },
          rowFunc: (dataTable, row, hasKeyServerStats) => {
            if (hasKeyServerStats) {
              dataTable.addRow([
                utcDate(row.date),
                row.data.codes_issued,
                row.data.codes_claimed,
                row.data.codes_invalid,
                row.data.tokens_claimed,
                row.data.total_publish_requests,
              ]);
            } else {
              dataTable.addRow([
                utcDate(row.date),
                row.data.codes_issued,
                row.data.codes_claimed,
                row.data.codes_invalid,
                row.data.tokens_claimed,
              ]);
            }
          },
        },
        {
          chartType: 'LineChart',
          chartDiv: '#user_issued_realm_chart_div',
          dashboardDiv: '#user_issued_dashboard_div',
          filterDiv: '#user_issued_realm_filter_div',
          headerFunc: (dataTable, hasKeyServerStats) => {
            dataTable.addColumn('date', 'Date');
            dataTable.addColumn('number', 'User Report Codes Issued');
            dataTable.addColumn('number', 'User Report Codes Claimed');
            dataTable.addColumn('number', 'User Report Tokens Claimed');
            dataTable.addColumn('number', 'User Report Device Mismatch');
          },
          rowFunc: (dataTable, row, hasKeyServerStats) => {
            dataTable.addRow([
              utcDate(row.date),
              row.data.user_reports_issued,
              row.data.user_reports_claimed,
              row.data.user_report_tokens_claimed,
              row.data.user_reports_invalid_nonce,
            ]);
          },
        },
        {
          chartType: 'AreaChart',
          chartDiv: '#comparison_chart_div',
          dashboardDiv: '#comparison_dashboard_div',
          filterDiv: '#comparison_filter_div',
          headerFunc: (dataTable, hasKeyServerStats) => {
            dataTable.addColumn('date', 'Date');
            dataTable.addColumn('number', 'System Issued Tokens Claimed');
            dataTable.addColumn('number', 'User Report Tokens Claimed');
          },
          rowFunc: (dataTable, row, hasKeyServerStats) => {
            dataTable.addRow([
              utcDate(row.date),
              row.data.tokens_claimed - row.data.user_report_tokens_claimed,
              row.data.user_report_tokens_claimed,
            ]);
          },
        },
        {
          chartType: 'AreaChart',
          chartDiv: '#invalid_chart_div',
          dashboardDiv: '#invalid_dashboard_div',
          filterDiv: '#invalid_filter_div',
          headerFunc: (dataTable, hasKeyServerStats) => {
            dataTable.addColumn('date', 'Date');
            dataTable.addColumn('number', 'Unknown');
            dataTable.addColumn('number', 'iOS');
            dataTable.addColumn('number', 'Android');
          },
          rowFunc: (dataTable, row, hasKeyServerStats) => {
            dataTable.addRow([
              utcDate(row.date),
              row.data.codes_invalid_by_os.unknown_os,
              row.data.codes_invalid_by_os.ios,
              row.data.codes_invalid_by_os.android,
            ]);
          },
        },
      ];

      const dateFormatter = new google.visualization.DateFormat({
        pattern: 'MMM dd',
      });

      for (let i = 0; i < charts.length; i++) {
        const chart = charts[i];
        const chartContainer = document.querySelector(chart.chartDiv);
        const dashboardContainer = document.querySelector(chart.dashboardDiv);
        const filterContainer = document.querySelector(chart.filterDiv);

        if (!chartContainer || !dashboardContainer || !filterContainer) {
          continue;
        }

        if (!data || !data.statistics) {
          const pContainer = chartContainer.querySelector('p');
          pContainer.innerText = 'No data yet.';
          continue;
        }

        const hasKeyServerStats = data.has_key_server_stats;
        const win = Math.min(30, data.statistics.length - 1);
        const startChart = new Date(data.statistics[win].date);

        const dataTable = new google.visualization.DataTable();
        chart.headerFunc(dataTable, hasKeyServerStats);
        for (let j = 0; j < data.statistics.length; j++) {
          const stat = data.statistics[j];
          chart.rowFunc(dataTable, stat, hasKeyServerStats);
        }
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

        const realmChart = new google.visualization.ChartWrapper({
          chartType: chart.chartType,
          containerId: chartContainer,
          options: {
            colors: ['#007bff', '#28a745', '#dc3545', '#6c757d', '#ee8c00'],
            chartArea: {
              left: 60,
              right: 40,
              bottom: 5,
              top: 40,
              width: '100%',
              height: '300',
            },
            isStacked: true,
            hAxis: { textPosition: 'none' },
            legend: { position: 'top' },
            width: '100%',
          },
        });

        dashboard.bind(filter, realmChart);
        dashboard.draw(dataTable);
        debounce('resize', async () => dashboard.draw(dataTable));
      }
    }

    function drawMeanClaimAgeChart(data) {
      const chartContainer = document.querySelector('#mean_claim_age_chart_div');
      const filterContainer = document.querySelector('#mean_claim_age_filter_div');

      if (!data || !data.statistics) {
        const pContainer = chartContainer.querySelector('p');
        pContainer.innerText = 'No data yet.';
        return;
      }

      const win = Math.min(30, data.statistics.length - 1);
      const startChart = new Date(data.statistics[win].date);

      const dataTable = new google.visualization.DataTable();
      dataTable.addColumn('date', 'Date');
      dataTable.addColumn('number', 'Mean code claim time');

      for (let i = 0; i < data.statistics.length; i++) {
        const stat = data.statistics[i];
        // convert seconds to minutes
        dataTable.addRow([utcDate(stat.date), stat.data.code_claim_mean_age_seconds / 60.0]);
      }

      const dateFormatter = new google.visualization.DateFormat({
        pattern: 'MMM dd',
      });
      dateFormatter.format(dataTable, 0);

      const dashboard = new google.visualization.Dashboard(chartContainer);

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
              hAxis: { format: 'M/d' },
            },
            chartView: {
              columns: [0, 1],
            },
            minRangeSize: 86400000, // ms for 1 day
          },
        },
      });

      const realmChart = new google.visualization.ChartWrapper({
        chartType: 'LineChart',
        containerId: chartContainer,
        options: {
          colors: ['#007bff'],
          chartArea: {
            left: 60,
            right: 40,
            bottom: 5,
            top: 40,
            width: '100%',
            height: '300',
          },
          hAxis: { textPosition: 'none' },
          vAxis: { title: 'Minutes' },
          legend: { position: 'top' },
          width: '100%',
        },
      });

      dashboard.bind(filter, realmChart);
      dashboard.draw(dataTable);
      debounce('resize', async () => dashboard.draw(dataTable));
    }

    function drawClaimAgeChart(data) {
      const chartContainer = document.querySelector('#issue_age_dist_chart_div');

      if (!data || !data.statistics || !data.statistics[0].data.code_claim_age_distribution) {
        const pContainer = chartContainer.querySelector('p');
        pContainer.innerText = 'No data yet.';
        return;
      }
      const statistics = data.statistics;

      issueAgeSlider.setAttribute('min', Math.min(smoothDrop.value, statistics.length));
      issueAgeSlider.setAttribute('max', statistics.length);
      issueAgeSlider.value = statistics.length;

      let vAxisMax = 1;
      for (let i = 0; i < statistics.length; i++) {
        const dataTable = getClaimAgeDataTable(statistics, i);
        for (let j = 0; j < dataTable.getNumberOfColumns(); j++) {
          const result = dataTable.getColumnRange(j);
          if (result && result.max && result.max > vAxisMax) {
            vAxisMax = result.max;
          }
        }
      }

      const ageOptions = {
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
          title: 'Time from issue to claim',
          gridlines: { color: 'transparent' },
          ticks: [
            { v: 1, f: '<1m' },
            { v: 2, f: '5m' },
            { v: 3, f: '15m' },
            { v: 4, f: '30m' },
            { v: 5, f: '1h' },
            { v: 6, f: '2h' },
            { v: 7, f: '3h' },
            { v: 8, f: '6h' },
            { v: 9, f: '12h' },
            { v: 10, f: '24h' },
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

      const ageChart = new google.visualization.ColumnChart(chartContainer);

      const drawChart = async () => {
        const idx = statistics.length - issueAgeSlider.value;
        if (idx < 0) {
          idx = 0;
        }

        const date = utcDate(statistics[idx].date);
        ageOptions.title = `${smoothDrop.value} days from ${date.toLocaleDateString()}`;
        const ageData = getClaimAgeDataTable(statistics, idx);
        ageChart.draw(ageData, ageOptions);
      };

      debounce('resize', drawChart);
      addEventListeners(issueAgeSlider, 'input', drawChart);
      drawChart();
    }

    const ageCache = new Map();
    function getClaimAgeDataTable(data, idx) {
      const key = `s${smoothDrop.value}i${idx}`;
      if (ageCache.has(key)) {
        return ageCache.get(key);
      }

      const dataTable = new google.visualization.DataTable();
      dataTable.addColumn('number', 'days');
      dataTable.addColumn('number', 'count');
      dataTable.addColumn({ type: 'string', role: 'style' });
      dataTable.addColumn({ type: 'string', role: 'annotation' });
      dataTable.addColumn({ type: 'string', role: 'tooltip', p: { html: true } });

      // sum over last ${smoothDrop.value} days
      const table = new Array(data[idx].data.code_claim_age_distribution.length).fill(0);
      let total = 0;
      // For the number of days to smooth over.
      for (let offset = 0; offset < smoothDrop.value; offset++) {
        if (idx + offset >= data.length) {
          break;
        }
        const row = data[idx + offset].data;
        for (let j = 0; j < row.code_claim_age_distribution.length; j++) {
          const codes = parseInt(row.code_claim_age_distribution[j]);
          table[j] += codes;
          total += codes;
        }
      }

      for (let i = 0; i < table.length; i++) {
        const r = [i + 1, table[i], '', '', ''];
        if (i == 9) {
          r[2] = '#6c757d';
          r[3] = '>1 day';
        }

        r[4] = `
          <ul class="list-unstyled text-nowrap m-0 p-3">
            <li>${r[1]} (${((r[1] / total) * 100).toPrecision(3)}%)</li>
          </ul>
        `;

        dataTable.addRow(r);
      }
      ageCache.set(key, dataTable);
      return dataTable;
    }
  });
})();

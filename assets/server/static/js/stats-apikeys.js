(() => {
  window.addEventListener('load', async (event) => {
    const chartContainer = document.querySelector('div#apikey_stats_chart');
    if (!chartContainer) {
      return;
    }
    const apiKeyID = chartContainer.dataset.apikeyId;

    google.charts.load('current', {
      packages: ['corechart', 'controls'],
      callback: drawChart,
    });

    function drawChart() {
      const request = new XMLHttpRequest();
      request.open('GET', `/stats/realm/api-keys/${apiKeyID}.json`);
      request.overrideMimeType('application/json');

      request.onload = (event) => {
        const pContainer = chartContainer.querySelector('p');

        const data = JSON.parse(request.response);
        if (!data.statistics || !data.statistics[0]) {
          pContainer.innerText = 'There is no data yet.';
          return;
        }

        switch (data.authorized_app_type) {
          case 'admin':
            drawAdminStats(data);
            break;
          case 'device':
            drawDeviceStats(data);
            break;
          case 'stats':
            pContainer.innerText = 'Statistics API keys do not produce statistics.';
            break;
          default:
            pContainer.innertText = 'Unknown API key type.';
            break;
        }
      };

      request.onerror = (event) => {
        console.error('error from response: ' + request.response);
        flash.error('Failed to render api key stats: ' + err);
      };

      request.send();
    }

    function drawAdminStats(data) {
      const dataTable = new google.visualization.DataTable();
      dataTable.addColumn('date', 'Date');
      dataTable.addColumn('number', 'Issued');

      for (let i = 0; i < data.statistics.length; i++) {
        const stat = data.statistics[i];
        dataTable.addRow([utcDate(stat.date), stat.data.codes_issued]);
      }

      const dateFormatter = new google.visualization.DateFormat({
        pattern: 'MMM dd',
      });
      dateFormatter.format(dataTable, 0);

      const options = {
        colors: ['#28a745', '#dc3545'],
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
      };

      const chart = new google.visualization.LineChart(chartContainer);
      chart.draw(dataTable, options);
      debounce('resize', async () => chart.draw(dataTable, options));
    }

    function drawDeviceStats(data) {
      const dataTable = new google.visualization.DataTable();
      dataTable.addColumn('date', 'Date');
      dataTable.addColumn('number', 'Codes claimed');
      dataTable.addColumn('number', 'Codes invalid');
      dataTable.addColumn('number', 'Tokens claimed');
      dataTable.addColumn('number', 'Tokens invalid');

      for (let i = 0; i < data.statistics.length; i++) {
        const stat = data.statistics[i];
        dataTable.addRow([
          utcDate(stat.date),
          stat.data.codes_claimed,
          stat.data.codes_invalid,
          stat.data.tokens_claimed,
          stat.data.tokens_invalid,
        ]);
      }

      const dateFormatter = new google.visualization.DateFormat({
        pattern: 'MMM dd',
      });
      dateFormatter.format(dataTable, 0);

      const options = {
        colors: ['#28a745', '#dc3545', '#17a2b8', '#ffc107'],
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
      };

      const chart = new google.visualization.LineChart(chartContainer);
      chart.draw(dataTable, options);
      debounce('resize', async () => chart.draw(dataTable, options));
    }
  });
})();

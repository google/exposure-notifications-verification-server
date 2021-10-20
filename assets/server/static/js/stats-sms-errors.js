(() => {
  window.addEventListener('load', (event) => {
    const dashboardContainer = document.querySelector('div#sms_errors_dashboard');
    if (!dashboardContainer) {
      return;
    }

    const chartContainer = dashboardContainer.querySelector('#sms_errors_chart');
    if (!chartContainer) {
      throw new Error('missing chart container for sms error stats');
    }

    const chartFilter = dashboardContainer.querySelector('.chart-filter');
    if (!chartFilter) {
      throw new Error('missing chart filter for sms error stats');
    }

    let chartData = [];
    $(() => redrawCharts(chartData, 300));

    google.charts.load('current', {
      packages: ['corechart', 'controls'],
      callback: drawChart,
    });

    function drawChart() {
      let request = new XMLHttpRequest();
      request.open('GET', '/stats/realm/sms-errors.json');
      request.overrideMimeType('application/json');

      request.onload = (event) => {
        let pContainer = chartContainer.querySelector('p');

        let data = JSON.parse(request.response);
        if (!data.statistics) {
          pContainer.innerText = 'There is no sms error data yet.';
          return;
        }

        const dataTable = new google.visualization.DataTable();
        dataTable.addColumn('date', 'Date');
        dataTable.addColumn('number', 'Errors');

        for (let i = 0; i < data.statistics.length; i++) {
          const stat = data.statistics[i];

          let total = 0;
          for (let j = 0; j < stat.error_data.length; j++) {
            total += stat.error_data[j].quantity;
          }

          dataTable.addRow([utcDate(stat.date), total]);
        }

        const tenDaysAgo = new Date(data.statistics[data.statistics.length - 10].date);

        let dateFormatter = new google.visualization.DateFormat({
          pattern: 'MMM dd',
        });
        dateFormatter.format(dataTable, 0);

        let dashboard = new google.visualization.Dashboard(dashboardContainer);

        let filter = new google.visualization.ControlWrapper({
          controlType: 'ChartRangeFilter',
          containerId: chartFilter,
          state: {
            range: {
              start: tenDaysAgo,
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

        let realmChart = new google.visualization.ChartWrapper({
          chartType: 'LineChart',
          containerId: chartContainer,
          options: {
            colors: ['#dc3545'],
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
        chartData.push({
          chart: dashboard,
          data: dataTable,
        });
      };

      request.onerror = (event) => {
        console.error('error from response: ' + request.response);
        flash.error('Failed to render sms error stats: ' + err);
      };

      request.send();
    }
  });
})();

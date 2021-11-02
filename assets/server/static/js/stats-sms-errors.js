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

    google.charts.load('current', {
      packages: ['corechart', 'controls'],
      callback: drawChart,
    });

    function drawChart() {
      const request = new XMLHttpRequest();
      request.open('GET', '/stats/realm/sms-errors.json');
      request.overrideMimeType('application/json');

      request.onload = (event) => {
        const pContainer = chartContainer.querySelector('p');

        const data = JSON.parse(request.response);
        if (!data.statistics || !data.statistics[0].error_data) {
          pContainer.innerText = 'There is no sms error data yet.';
          return;
        }

        const dataTable = new google.visualization.DataTable();
        dataTable.addColumn('date', 'Date');

        for (let i = 0; i < data.statistics.length; i++) {
          const stat = data.statistics[i];

          const row = [utcDate(stat.date)];
          for (let j = 0; j < stat.error_data.length; j++) {
            const errorData = stat.error_data[j];

            // On the first row, extract the column headers.
            if (i === 0) {
              const label = errorData.error_code;
              dataTable.addColumn('number', label);
            }

            row.push(errorData.quantity);
          }

          dataTable.addRow(row);
        }

        const startChart = new Date(data.statistics[30].date);

        const dateFormatter = new google.visualization.DateFormat({
          pattern: 'MMM dd',
        });
        dateFormatter.format(dataTable, 0);

        const dashboard = new google.visualization.Dashboard(dashboardContainer);

        const filter = new google.visualization.ControlWrapper({
          controlType: 'ChartRangeFilter',
          containerId: chartFilter,
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
          chartType: 'ColumnChart',
          containerId: chartContainer,
          options: {
            colors: ['#dc3545', '#5c161d', '#e83849', '#c22f3d', '#9c2531'],
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
      };

      request.onerror = (event) => {
        console.error('error from response: ' + request.response);
        flash.error('Failed to render sms error stats: ' + err);
      };

      request.send();
    }
  });
})();

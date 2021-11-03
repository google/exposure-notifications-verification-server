(() => {
  window.addEventListener('load', async (event) => {
    const chartContainer = document.querySelector('div#user_stats_chart');
    if (!chartContainer) {
      return;
    }
    const userID = chartContainer.dataset.userId;

    google.charts.load('current', {
      packages: ['corechart'],
      callback: drawChart,
    });

    function drawChart() {
      const request = new XMLHttpRequest();
      request.open('GET', `/stats/realm/users/${userID}.json`);
      request.overrideMimeType('application/json');

      request.onload = (event) => {
        const pContainer = chartContainer.querySelector('p');

        const data = JSON.parse(request.response);
        if (!data.statistics || !data.statistics[0]) {
          pContainer.innerText = 'There is no data yet.';
          return;
        }

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
          colors: ['#007bff', '#ff7b00'],
          chartArea: {
            left: 40, // leave room for y-axis labels
            right: 20,
            width: '100%',
          },
          hAxis: { format: 'M/d' },
          legend: 'none',
          width: '100%',
        };

        const chart = new google.visualization.LineChart(chartContainer);
        chart.draw(dataTable, options);
        debounce('resize', async () => chart.draw(dataTable, options));
      };

      request.onerror = (event) => {
        console.error('error from response: ' + request.response);
        flash.error('Failed to render user stats: ' + err);
      };

      request.send();
    }
  });
})();

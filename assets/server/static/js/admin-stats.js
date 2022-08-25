(() => {
    window.addEventListener('load', async (event) => {
      const containerChart = document.querySelector('div#system_chart_div');
      if (!containerChart) {
        return;
      }
  
      google.charts.load('current', {
        packages: ['corechart', 'controls'],
        callback: drawCharts,
      });
  
      function drawCharts() {
        const request = new XMLHttpRequest();
        request.open('GET', '/admin/stats/system.json');
        request.overrideMimeType('application/json');
  
        request.onload = (event) => {
          const data = JSON.parse(request.response);
          drawSystemCodeChart(data);
        };
  
        request.onerror = (event) => {
          console.error('error from response: ' + request.response);
          flash.error('Failed to load realm stats: ' + err);
        };
  
        request.send();
      }
  
      function drawSystemCodeChart(data) {
        const charts = [
          {
            chartType: 'LineChart',
            chartDiv: '#system_chart_div',
            dashboardDiv: '#dashboard_div',
            filterDiv: '#filter_div',
            headerFunc: (dataTable, hasKeyServerStats) => {
              dataTable.addColumn('date', 'Date');
              dataTable.addColumn('number', 'Codes Issued');
              dataTable.addColumn('number', 'Codes Claimed');
            },
            rowFunc: (dataTable, row, hasKeyServerStats) => {
              if (hasKeyServerStats) {
                dataTable.addRow([
                  utcDate(row.date),
                  row.data.codes_issued,
                  row.data.codes_claimed,
                ]);
              } else {
                dataTable.addRow([
                  utcDate(row.date),
                  row.data.codes_issued,
                  row.data.codes_claimed,
                ]);
              }
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
    });
  })();
  
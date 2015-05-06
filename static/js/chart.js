google.load('visualization', '1', {packages: ['corechart', 'line']});
//google.setOnLoadCallback(drawChart);

function drawChart(duration) {
    duration = parseInt(duration, 10);
    if (duration <= 0) {
	duration = 600
    }
    $.getJSON("/json/temperature/"+duration+".json", function(rows){
	drawChartAux(rows);
    });
}

function drawChartAux(rows) {
    var data = new google.visualization.DataTable();
    data.addColumn('datetime', 'date');
    data.addColumn('number', 'ave');

    rows = rows.map(function(arr){return [new Date(arr['timestamp']),arr['ave']];});
    data.addRows(rows);

    var options = {
	vAxis: {minorGridlines: {count: 4} },
	width: 1000,
	height: 500,
    };

    var chart = new google.visualization.LineChart(document.getElementById('chart'));
    chart.draw(data, options);
}

$(window).load(function() {
    // select third list item
    $("li:first").addClass("active");
    drawChart($("li:first").data("duration"));
    
    // dynamically activate list items when clicked
    $(".nav.nav-pills li").on("click",function(){
	$(".nav.nav-pills li").removeClass("active");
	drawChart($(this).data("duration"));
	$(this).addClass("active");
    });
});

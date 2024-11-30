default:

update_example_post:
	curl -X POST --data-binary @example/postmap.json -o example/postmap.png localhost:5000/map.png

update_example_get:
	curl -o example/map.png 'localhost:5000/map.png?center=53.5438,9.9768&zoom=15&size=800x500&markers=color:blue|53.54129165,9.98420576699353&markers=color:yellow|53.54565525,9.9680555636958&markers=size:tiny|color:red|53.54846472989871,9.978977621091543'

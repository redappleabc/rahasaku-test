SELECT category, COUNT(DISTINCT menus.id) AS count FROM menus 
JOIN muscle_menus ON menus.id = muscle_menus.menu_id 
JOIN muscles ON muscle_menus.muscle_id = muscles.id 
WHERE muscles.name IN ('muscle1', 'muscle2') 
GROUP BY category 
ORDER BY `count` DESC;


SELECT category, COUNT(DISTINCT menus.id) AS COUNT FROM menus 
JOIN muscle_menus ON menus.id = muscle_menus.menu_id 
JOIN (SELECT id FROM muscles WHERE `name` IN ('muscle1', 'muscle2')) AS filtered_muscles ON muscle_menus.muscle_id = filtered_muscles.id 
GROUP BY category 
ORDER BY `count` DESC;


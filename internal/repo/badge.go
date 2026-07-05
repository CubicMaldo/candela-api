package repo

// badgeCaseSQL returns the SQL CASE expression that maps an accumulated
// points expression to the Candela badge tiers (functional spec §14.5).
// pointsExpr must evaluate to the user's resulting points_total.
func badgeCaseSQL(pointsExpr string) string {
	return `CASE
	    WHEN ` + pointsExpr + ` >= 5000 THEN 'Leyenda'
	    WHEN ` + pointsExpr + ` >= 1500 THEN 'Teso'
	    WHEN ` + pointsExpr + ` >= 500  THEN 'Parroquiano'
	    WHEN ` + pointsExpr + ` >= 150  THEN 'Callejero'
	    ELSE 'Nuevo en la vuelta'
	  END`
}

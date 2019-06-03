package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog"

	_ "github.com/mattn/go-sqlite3"
)

func createLogger() *zerolog.Logger {
	writer := zerolog.NewConsoleWriter()
	writer.TimeFormat = time.RFC3339
	writer.FormatCaller = func(i interface{}) string {
		if i == nil {
			return ""
		}
		value := fmt.Sprintf("%v", i)
		// далее использутся только репозитории из github и gitlab, отщивыаем всё вплоть до этих слов
		pos := strings.Index(value, "github.com")
		if pos < 0 {
			pos = strings.Index(value, "gitlab.")
			if pos >= 0 {
				value = value[pos:]
			}
		} else {
			value = value[pos:]
		}
		return "(" + value + ")"
	}
	writer.FormatMessage = func(i interface{}) string {
		return fmt.Sprintf("\033[1m%v\033[0m", i)
	}
	writer.FormatTimestamp = func(i interface{}) string {
		if i == nil {
			return ""
		}
		return fmt.Sprintf("\033[33;1m%v\033[0m", i)
	}
	writer.FormatFieldName = func(i interface{}) string {
		return fmt.Sprintf("\033[35m%s\033[0m", i)
	}
	writer.FormatFieldValue = func(i interface{}) string {
		return fmt.Sprintf("[%v]", i)
	}
	writer.FormatErrFieldName = func(i interface{}) string {
		return fmt.Sprintf("\033[31m%s\033[0m", i)
	}
	writer.FormatErrFieldValue =
		func(i interface{}) string {
			return fmt.Sprintf("\033[31m[%v]\033[0m", i)
		}
	log := zerolog.New(writer).Level(zerolog.DebugLevel).With().Caller().Timestamp().Logger()
	return &log
}

func noAll(input string) string {
	if strings.HasSuffix(input, " all") {
		return input[:len(input)-4]
	}
	return input
}

func skillURL(href string) string {
	return "https://thunderskill.com" + href
}

func countryOnly(country string) string {
	if strings.HasPrefix(country, "country_") {
		return country[8:]
	}
	return country
}

type battleStat struct {
	PerBattle *float64
	PerDeath  *float64
}

type gameClassStat struct {
	BattleRating uint
	Battles      uint
	WinRate      float64
	Downs        battleStat
	Kills        battleStat
}

type totalStat struct {
	Role       string
	Country    string
	Name       string
	Rank       uint
	Arcade     *gameClassStat
	Realistic  *gameClassStat
	Simulation *gameClassStat
}

func extractCount(logger zerolog.Logger, input string) *uint {
	input = strings.TrimSpace(input)
	if len(input) == 0 || input == "N/A" {
		return nil
	}
	res, err := strconv.ParseUint(input, 10, 64)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to extract statistics value")
	}
	result := uint(res)
	return &result
}

func extractRate(logger zerolog.Logger, input string) *float64 {
	input = strings.TrimSpace(input)
	if len(input) == 0 || input == "N/A" {
		return nil
	}
	res, err := strconv.ParseFloat(input, 64)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to extract statics rate value")
	}
	return &res
}

func statLogger(logger zerolog.Logger, statName string) zerolog.Logger {
	return logger.With().Str("stat", statName).Logger()
}

func getVehicleStats(log zerolog.Logger, href string) (res totalStat) {
	resp, err := http.Get(skillURL(href))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get a vehicle page")
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse vehicle page")
	}

	doc.Find(".row").Find(".mt-5").Children().Each(func(i int, selection *goquery.Selection) {
		var stat gameClassStat
		var logger zerolog.Logger
		var noStat bool
		switch i {
		case 0:
			logger = log.With().Str("game-type", "arcade").Logger()
		case 1:
			logger = log.With().Str("game-type", "realistic").Logger()
		case 2:
			logger = log.With().Str("game-type", "simulation").Logger()

		}
		selection.Find("ul.stats").Find("li").Each(func(i int, selection *goquery.Selection) {
			if i&1 != 0 || noStat {
				return
			}
			switch i / 2 {
			case 0:
				// Battles
				log := statLogger(logger, "battles-count")
				selection.Find(".badge").Each(func(i int, selection *goquery.Selection) {
					res := extractCount(log, selection.Text())
					if res == nil {
						log.Warn().Msg("no stat found")
						noStat = true
						return
					}
					stat.Battles = *res
				})
			case 1:
				// Win rate
				log := statLogger(logger, "win-rate")
				selection.Find(".badge").Each(func(i int, selection *goquery.Selection) {
					res := strings.TrimSpace(selection.Text())
					if len(res) == 0 || res == "N/A" {
						return
					}
					res = strings.Trim(res, "%")
					val, err := strconv.ParseFloat(res, 64)
					if err != nil {
						log.Fatal().Err(err).Msg("failed to extract win rate")
						return
					}
					stat.WinRate = val
				})
			case 2:
				// Aircrafts downs per battle
				log := statLogger(logger, "aircrafts-per-battle")
				selection.Find(".badge").Each(func(i int, selection *goquery.Selection) {
					stat.Downs.PerBattle = extractRate(log, selection.Text())
				})
			case 3:
				// Aircrafts downs per death
				log := statLogger(logger, "aircrafts-per-death")
				selection.Find(".badge").Each(func(i int, selection *goquery.Selection) {
					stat.Downs.PerDeath = extractRate(log, selection.Text())
				})
			case 4:
				// Tanks downs per battle
				log := statLogger(logger, "tanks-per-battle")
				selection.Find(".badge").Each(func(i int, selection *goquery.Selection) {
					stat.Kills.PerBattle = extractRate(log, selection.Text())
				})
			case 5:
				// Tanks downs per death
				log := statLogger(logger, "tanks-per-death")
				selection.Find(".badge").Each(func(i int, selection *goquery.Selection) {
					stat.Kills.PerDeath = extractRate(log, selection.Text())
				})
			default:
				logger.Warn().Err(fmt.Errorf("unexpected game class statistics found")).Msg(selection.Text())
			}
		})
		if !noStat {
			switch i {
			case 0:
				res.Arcade = &stat
			case 1:
				res.Realistic = &stat
			case 2:
				res.Simulation = &stat
			default:
				logger.Fatal().Err(fmt.Errorf("unknown game class listed")).Msg(selection.Text())
			}
		}
	})

	doc.Find("ul.params").Each(func(i int, paramsSelection *goquery.Selection) {
		switch i {
		case 0:
			paramsSelection.Find("span.param_value").Find("strong").Each(func(i int, selection *goquery.Selection) {
				switch i {
				case 0:
					// not needed
				case 1:
					// not needed
				case 2:
					// battle rank
					log := statLogger(log, "rank")
					val := extractCount(log, selection.Text())
					if val == nil {
						log.Fatal().Err(fmt.Errorf("no rank found")).Msg("failed to extract vehicle data")
					} else {
						res.Rank = *val
					}
				default:
					paramsSelection.Find("span.param_name").Find("strong").Each(func(j int, selection *goquery.Selection) {
						if j != i {
							return
						}
						log.Warn().Err(fmt.Errorf("unsupported parameter `%s`", strings.TrimSpace(selection.Text())))
					})
				}
			})
		case 1:
			return
		case 2:
			paramsSelection.Find("span.param_value").Each(func(i int, selection *goquery.Selection) {
				var value string
				selection.Find("strong").Each(func(i int, selection *goquery.Selection) {
					value = strings.TrimSpace(selection.Text())
				})
				val, err := strconv.ParseFloat(value, 64)
				if err != nil {
					log.Error().Err(err).Msg("failed to extract battle rating")
				}
				val = (val + 0.05) * 10
				switch i {
				case 0:
					if res.Arcade != nil {
						res.Arcade.BattleRating = uint(val)
					}
				case 1:
					if res.Realistic != nil {
						res.Realistic.BattleRating = uint(val)
					}
				case 2:
					if res.Simulation != nil {
						res.Simulation.BattleRating = uint(val)
					}
				default:
					paramsSelection.Find("span.param_name").Find("strong").Each(func(j int, selection *goquery.Selection) {
						if j != i {
							return
						}
						log.Warn().Err(fmt.Errorf("unsupported parameter `%s`", strings.TrimSpace(selection.Text())))
					})
				}
			})
		default:
			return
		}
	})

	return
}

func dumpStat(conn *sql.DB, stat totalStat, count int) error {
	values := []interface{}{
		count, stat.Name, stat.Role, stat.Country, stat.Rank,
	}
	if stat.Arcade != nil {
		values = append(values,
			stat.Arcade.BattleRating,
			stat.Arcade.Battles,
			stat.Arcade.WinRate,
			stat.Arcade.Downs.PerBattle, stat.Arcade.Downs.PerDeath,
			stat.Arcade.Kills.PerBattle, stat.Arcade.Kills.PerDeath,
		)
	} else {
		values = append(values, nil, nil, nil, nil, nil, nil, nil)
	}
	if stat.Realistic != nil {
		values = append(values,
			stat.Realistic.BattleRating,
			stat.Realistic.Battles,
			stat.Realistic.WinRate,
			stat.Realistic.Downs.PerBattle, stat.Realistic.Downs.PerDeath,
			stat.Realistic.Kills.PerBattle, stat.Realistic.Kills.PerDeath,
		)
	} else {
		values = append(values, nil, nil, nil, nil, nil, nil, nil)
	}
	if stat.Simulation != nil {
		values = append(values,
			stat.Simulation.BattleRating,
			stat.Simulation.Battles,
			stat.Simulation.WinRate,
			stat.Simulation.Downs.PerBattle, stat.Simulation.Downs.PerDeath,
			stat.Simulation.Kills.PerBattle, stat.Simulation.Kills.PerDeath,
		)
	} else {
		values = append(values, nil, nil, nil, nil, nil, nil, nil)
	}

	_, err := conn.Exec(`
INSERT INTO tskill (id, name, role, country, rank,
	arcadeBR, arcadeBattles, arcadeWinRate, arcadeDownsPerBattle, arcadeDownsPerDeath, arcadeKillsPerBattle, arcadeKillsPerDeath,
	realisticBR, realisticBattles, realisticWinRate, realisticDownsPerBattle, realisticDownsPerDeath, realisticKillsPerBattle, realisticKillsPerDeath,
	simulationBR, simulationBattles, simulationWinRate, simulationDownsPerBattle, simulationDownsPerDeath, simulationKillsPerBattle, simulationKillsPerDeath
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, values...)
	if err != nil {
		return fmt.Errorf("failed to insert stats: %s", err)
	}

	return nil
}

func main() {
	log := createLogger()

	resp, err := http.Get(skillURL("/en/vehicles"))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get a list of vehicles")
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse a list of vehicles")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get a home dir")
	}
	dest := filepath.Join(home, "thunderskill.db")
	if err := os.Remove(dest); err != nil {
		if !os.IsNotExist(err) {
			log.Fatal().Err(err).Msg("failed to remove a previous dump")
		}
	}
	conn, err := sql.Open("sqlite3", fmt.Sprintf("file:%s", dest))
	_, err = conn.Exec(`CREATE TABLE tskill ( 
	id		                  int primary key not null,
	name	                  text            not null,
	role	                  text            not null,
	country					  text 			  not null,
	rank	                  int             not null,
	arcadeBR                  int,
	arcadeBattles	          int,
	arcadeWinRate             float,
	arcadeDownsPerBattle     float,
	arcadeDownsPerDeath      float,
	arcadeKillsPerBattle     float,
	arcadeKillsPerDeath      float,
	realisticBR               int,
	realisticBattles	      int,
	realisticWinRate          float,
	realisticDownsPerBattle  float,
	realisticDownsPerDeath   float,
	realisticKillsPerBattle  float,
	realisticKillsPerDeath   float,
	simulationBR              int,
	simulationBattles	      int,
	simulationWinRate         float,
	simulationDownsPerBattle float,
	simulationDownsPerDeath  float,
	simulationKillsPerBattle float,
	simulationKillsPerDeath  float
)`)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create dump table")
	}
	defer conn.Close()

	var count int
	doc.Find(`tr[data-role]`).Each(func(i int, selection *goquery.Selection) {
		role := noAll(selection.AttrOr("data-role", ""))
		country := countryOnly(noAll(selection.AttrOr("data-country", "")))
		var href string
		var name string
		selection.Find(".vehicle").Each(func(i int, selection *goquery.Selection) {
			name = selection.AttrOr("data-sort", "unknown")
			href = selection.Find("a[href]").AttrOr("href", "")
		})
		name = noAll(name)
		stat := getVehicleStats(log.With().Str("vehicle", name).Logger(), href)
		stat.Name = name
		stat.Role = role
		stat.Country = country

		logger := log.With().Str("reference", href).Str("name", name).Str("country", country).Str("role", role).Logger()
		if err := dumpStat(conn, stat, count); err != nil {
			logger.Fatal().Err(err).Msg("failed to dump vehicle data")
		}
		count++
		logger.Info().Msg("done")
	})
}

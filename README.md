# CityNext

A lightweight Go service for managing civic appointments with public holiday awareness and date validation.

## ⚙️ Setup

Clone the repository and run the server with a target year:

```bash
git clone git@github.com:dmaclay/CityNext.git
cd CityNext

# Run the server for a specific year (e.g. 2075)
go run main.go 2075


# To test:
go test -v

## 🧪 Test Suite Overview

This test suite validates the core logic of the `/appointments` API by simulating HTTP POST requests. It uses an in-memory SQLite database and manually injected UK public holidays for the year 2075.

### ✅ Covered Scenarios

| Test Name                  | Description                                                                 |
|---------------------------|-----------------------------------------------------------------------------|
| `TestOutOfYearDate`       | Rejects appointments outside the configured year (e.g., 2074)               |
| `TestEarlyYearDate`       | Rejects appointments earlier than the simulated "today" (e.g., Jan 1)       |
| `TestPublicHolidayClash`  | Rejects appointments on known public holidays (e.g., Christmas Day)         |
| `TestValidAppointment`    | Accepts a valid appointment on a non-holiday future date                    |
| `TestDuplicateAppointment`| Rejects duplicate appointments for the same date                            |
| `TestInvalidDateFormat`   | Rejects malformed date strings (e.g., using slashes instead of dashes)      |
| `TestMissingFirstName`    | Rejects requests missing the `firstName` field                              |
| `TestMissingLastName`     | Rejects requests missing the `lastName` field                               |

### 🗓️ Public Holidays Used in Tests

The following UK holidays for 2075 are hardcoded into the test server:

- 2075-01-01 – New Year's Day  
- 2075-01-02 – 2 January  
- 2075-03-18 – Saint Patrick's Day  
- 2075-04-05 – Good Friday  
- 2075-04-08 – Easter Monday  
- 2075-05-06 – Early May Bank Holiday  
- 2075-05-27 – Spring Bank Holiday  
- 2075-07-12 – Battle of the Boyne  
- 2075-08-05 – Summer Bank Holiday  
- 2075-08-26 – Summer Bank Holiday  
- 2075-12-02 – Saint Andrew's Day  
- 2075-12-25 – Christmas Day  
- 2075-12-26 – Boxing Day


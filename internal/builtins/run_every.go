/*
 * ALR Updater - Automated updater bot for ALR packages
 * Copyright (C) 2025 The ALR Authors
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package builtins

import (
	"sync"
	"time"

	"go.elara.ws/logger/log"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

var (
	tickerMtx   = &sync.Mutex{}
	tickerCount = 0
	tickers     = map[int]*time.Ticker{}
	// Для хранения зарегистрированных функций
	registeredFunctions = map[string]*starlark.Function{}
	registeredFnMtx     = &sync.RWMutex{}
)

func runEvery(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var every string
	var fn *starlark.Function
	err := starlark.UnpackArgs("run_every", args, kwargs, "every", &every, "function", &fn)
	if err != nil {
		return nil, err
	}

	d, err := time.ParseDuration(every)
	if err != nil {
		return nil, err
	}

	tickerMtx.Lock()
	t := time.NewTicker(d)
	handle := tickerCount
	tickers[handle] = t
	tickerCount++
	tickerMtx.Unlock()
	log.Debug("Created new ticker").Int("handle", handle).Str("duration", every).Stringer("pos", thread.CallFrame(1).Pos).Send()

	go func() {
		for range t.C {
			log.Debug("Calling scheduled function").Str("name", fn.Name()).Stringer("pos", fn.Position()).Send()
			_, err := starlark.Call(thread, fn, nil, nil)
			if err != nil {
				log.Warn("Error while executing scheduled function").Str("name", fn.Name()).Stringer("pos", fn.Position()).Err(err).Send()
			}
		}
	}()

	return newTickerHandle(handle), nil
}

func sleep(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var duration string
	err := starlark.UnpackArgs("sleep", args, kwargs, "duration", &duration)
	if err != nil {
		return nil, err
	}

	d, err := time.ParseDuration(duration)
	if err != nil {
		return nil, err
	}

	log.Debug("Sleeping").Str("duration", duration).Stringer("pos", thread.CallFrame(1).Pos).Send()
	time.Sleep(d)
	return starlark.None, nil
}

func stopTicker(handle int) *starlark.Builtin {
	return starlark.NewBuiltin("stop", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		tickerMtx.Lock()
		tickers[handle].Stop()
		delete(tickers, handle)
		tickerMtx.Unlock()
		log.Debug("Stopped ticker").Int("handle", handle).Stringer("pos", thread.CallFrame(1).Pos).Send()
		return starlark.None, nil
	})
}

func newTickerHandle(handle int) starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"stop": stopTicker(handle),
	})
}

var runEveryModule = &starlarkstruct.Module{
	Name: "run_every",
	Members: starlark.StringDict{
		"minute": starlark.NewBuiltin("run_every.minute", runEveryMinute),
		"hour":   starlark.NewBuiltin("run_every.hour", runEveryHour),
		"day":    starlark.NewBuiltin("run_every.day", runEveryDay),
		"week":   starlark.NewBuiltin("run_every.week", runEveryWeek),
	},
}

func runEveryMinute(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fn *starlark.Function
	var count int64 = 1
	err := starlark.UnpackArgs("run_every.minute", args, kwargs, "function", &fn, "count?", &count)
	if err != nil {
		return nil, err
	}
	duration := time.Duration(count) * time.Minute
	return runScheduled(thread, fn, duration.String())
}

func runEveryHour(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fn *starlark.Function
	var count int64 = 1
	err := starlark.UnpackArgs("run_every.hour", args, kwargs, "function", &fn, "count?", &count)
	if err != nil {
		return nil, err
	}
	duration := time.Duration(count) * time.Hour
	return runScheduled(thread, fn, duration.String())
}

func runEveryDay(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fn *starlark.Function
	var count int64 = 1
	err := starlark.UnpackArgs("run_every.day", args, kwargs, "function", &fn, "count?", &count)
	if err != nil {
		return nil, err
	}
	duration := time.Duration(count) * 24 * time.Hour
	return runScheduled(thread, fn, duration.String())
}

func runEveryWeek(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fn *starlark.Function
	var count int64 = 1
	err := starlark.UnpackArgs("run_every.week", args, kwargs, "function", &fn, "count?", &count)
	if err != nil {
		return nil, err
	}
	duration := time.Duration(count) * 7 * 24 * time.Hour
	return runScheduled(thread, fn, duration.String())
}

func runScheduled(thread *starlark.Thread, fn *starlark.Function, duration string) (starlark.Value, error) {
	// Сохраняем функцию для возможности немедленного запуска
	registeredFnMtx.Lock()
	functionKey := thread.Name + ":" + fn.Name()
	registeredFunctions[functionKey] = fn
	registeredFnMtx.Unlock()

	d, err := time.ParseDuration(duration)
	if err != nil {
		return nil, err
	}

	tickerMtx.Lock()
	t := time.NewTicker(d)
	handle := tickerCount
	tickers[handle] = t
	tickerCount++
	tickerMtx.Unlock()
	log.Debug("Created new scheduled ticker").Int("handle", handle).Str("duration", duration).Stringer("pos", thread.CallFrame(1).Pos).Send()

	go func() {
		for range t.C {
			log.Debug("Calling scheduled function").Str("name", fn.Name()).Stringer("pos", fn.Position()).Send()
			_, err := starlark.Call(thread, fn, nil, nil)
			if err != nil {
				log.Warn("Error while executing scheduled function").Str("name", fn.Name()).Stringer("pos", fn.Position()).Err(err).Send()
			}
		}
	}()

	return newTickerHandle(handle), nil
}

// GetRegisteredFunctions возвращает все зарегистрированные функции
func GetRegisteredFunctions() map[string]*starlark.Function {
	registeredFnMtx.RLock()
	defer registeredFnMtx.RUnlock()
	
	result := make(map[string]*starlark.Function)
	for k, v := range registeredFunctions {
		result[k] = v
	}
	return result
}

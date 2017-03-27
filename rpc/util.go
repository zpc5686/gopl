/**********************************************************************************************************************
 *
 * Copyright (c) 2010 babeltime.com, Inc. All Rights Reserved
 * $Id$
 *
 **********************************************************************************************************************/

/**
 * @file $HeadURL$
 * @author $Author$(wuqilin@babeltime.com)
 * @date $Date$
 * @version $Revision$
 * @brief
 *
 **/

package rpc

import (
	"encoding/gob"
	"reflect"
	"unicode"
	"unicode/utf8"
)

func IsExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// Is this type exported or a builtin?
func IsExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return IsExported(t.Name()) || t.PkgPath() == ""
}

/*
让gob能够编解码map of interface  & slice of interface
*/
func SetGobForInterface() {
	gob.Register(make(map[string]interface{}))
	gob.Register(make([]interface{}, 0))
}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */

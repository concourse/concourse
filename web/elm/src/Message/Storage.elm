port module Message.Storage exposing
    ( Key
    , Value
    , loadFromLocalStorage
    , loadFromSessionStorage
    , receivedFromLocalStorage
    , receivedFromSessionStorage
    , saveToLocalStorage
    , saveToSessionStorage
    , sideBarStateKey
    , tokenKey
    )

import Json.Encode


type alias Key =
    String


type alias Value =
    String


port saveToLocalStorage : ( Key, Json.Encode.Value ) -> Cmd msg


port saveToSessionStorage : ( Key, Json.Encode.Value ) -> Cmd msg


port loadFromLocalStorage : Key -> Cmd msg


port loadFromSessionStorage : Key -> Cmd msg


port receivedFromLocalStorage : (( Key, Value ) -> msg) -> Sub msg


port receivedFromSessionStorage : (( Key, Value ) -> msg) -> Sub msg


sideBarStateKey : Key
sideBarStateKey =
    "is_sidebar_open"


tokenKey : Key
tokenKey =
    "csrf_token"

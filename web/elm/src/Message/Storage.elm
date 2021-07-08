port module Message.Storage exposing
    ( Key
    , Value
    , deleteFromCache
    , deleteFromLocalStorage
    , favoritedInstanceGroupsKey
    , favoritedPipelinesKey
    , jobsKey
    , loadFromCache
    , loadFromLocalStorage
    , pipelinesKey
    , receivedFromCache
    , receivedFromLocalStorage
    , saveToCache
    , saveToLocalStorage
    , sideBarStateKey
    , teamsKey
    , tokenKey
    )

import Json.Encode


type alias Key =
    String


type alias Value =
    Json.Encode.Value



-- Uses localStorage: https://developer.mozilla.org/en-US/docs/Web/API/Window/localStorage


port saveToLocalStorage : ( Key, Value ) -> Cmd msg


port deleteFromLocalStorage : Key -> Cmd msg


port loadFromLocalStorage : Key -> Cmd msg


port receivedFromLocalStorage : (( Key, Value ) -> msg) -> Sub msg



-- Uses the browser cache API: https://developer.mozilla.org/en-US/docs/Web/API/Cache


port saveToCache : ( Key, Value ) -> Cmd msg


port deleteFromCache : Key -> Cmd msg


port loadFromCache : Key -> Cmd msg


port receivedFromCache : (( Key, Value ) -> msg) -> Sub msg


sideBarStateKey : Key
sideBarStateKey =
    "side_bar_state"


tokenKey : Key
tokenKey =
    "csrf_token"


jobsKey : Key
jobsKey =
    "jobs"


pipelinesKey : Key
pipelinesKey =
    "pipelines"


teamsKey : Key
teamsKey =
    "teams"


favoritedPipelinesKey : Key
favoritedPipelinesKey =
    "favorited_pipelines"


favoritedInstanceGroupsKey : Key
favoritedInstanceGroupsKey =
    "favorited_instance_groups"

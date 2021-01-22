port module Message.Storage exposing
    ( Key
    , Value
    , deleteFromLocalStorage
    , favoritedInstanceGroupsKey
    , favoritedPipelinesKey
    , jobsKey
    , loadFromLocalStorage
    , pipelinesKey
    , receivedFromLocalStorage
    , saveToLocalStorage
    , sideBarStateKey
    , teamsKey
    , tokenKey
    )

import Json.Encode


type alias Key =
    String


type alias Value =
    String


port saveToLocalStorage : ( Key, Json.Encode.Value ) -> Cmd msg


port deleteFromLocalStorage : Key -> Cmd msg


port loadFromLocalStorage : Key -> Cmd msg


port receivedFromLocalStorage : (( Key, Value ) -> msg) -> Sub msg


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

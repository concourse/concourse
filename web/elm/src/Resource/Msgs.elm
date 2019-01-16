module Resource.Msgs exposing (Msg(..))

import Concourse
import Concourse.Pagination exposing (Page, Paginated)
import Http
import Resource.Models as Models
import Time exposing (Time)
import TopBar


type Msg
    = Noop
    | AutoupdateTimerTicked Time
    | ResourceFetched (Result Http.Error Concourse.Resource)
    | VersionedResourcesFetched (Maybe Page) (Result Http.Error (Paginated Concourse.VersionedResource))
    | LoadPage Page
    | ClockTick Time.Time
    | ExpandVersionedResource Int
    | InputToFetched Int (Result Http.Error (List Concourse.Build))
    | OutputOfFetched Int (Result Http.Error (List Concourse.Build))
    | NavTo String
    | TogglePinBarTooltip
    | ToggleVersionTooltip
    | PinVersion Int
    | UnpinVersion
    | VersionPinned (Result Http.Error ())
    | VersionUnpinned (Result Http.Error ())
    | ToggleVersion Models.VersionToggleAction Int
    | VersionToggled Models.VersionToggleAction Int (Result Http.Error ())
    | PinIconHover Bool
    | Hover Models.Hoverable
    | TopBarMsg TopBar.Msg
    | Check
    | Checked (Result Http.Error ())

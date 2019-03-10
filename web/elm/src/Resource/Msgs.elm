module Resource.Msgs exposing (Msg(..))

import Concourse.Pagination exposing (Page, Paginated)
import Resource.Models as Models
import Routes
import TopBar.Msgs


type Msg
    = LoadPage Page
    | ExpandVersionedResource Models.VersionId
    | NavTo Routes.Route
    | TogglePinBarTooltip
    | ToggleVersionTooltip
    | PinVersion Models.VersionId
    | UnpinVersion
    | ToggleVersion Models.VersionToggleAction Models.VersionId
    | PinIconHover Bool
    | Hover Models.Hoverable
    | CheckRequested Bool
    | TopBarMsg TopBar.Msgs.Msg
    | EditComment String
    | SaveComment String
    | FocusTextArea
    | BlurTextArea

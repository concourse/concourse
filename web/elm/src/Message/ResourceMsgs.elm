module Message.ResourceMsgs exposing (Msg(..))

import Concourse.Pagination exposing (Page, Paginated)
import Message.TopBarMsgs
import Resource.Models as Models
import Routes


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
    | TopBarMsg Message.TopBarMsgs.Msg
    | EditComment String
    | SaveComment String
    | FocusTextArea
    | BlurTextArea

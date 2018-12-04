module Dashboard.Msgs exposing (Cli(..), Msg(..))

import RemoteData
import Concourse
import Dashboard.APIData as APIData
import Http
import Time
import Keyboard
import NewTopBar


type Cli
    = OSX
    | Windows
    | Linux


type Msg
    = Noop
    | APIDataFetched (RemoteData.WebData ( Time.Time, ( APIData.APIData, Maybe Concourse.User ) ))
    | ClockTick Time.Time
    | AutoRefresh Time.Time
    | ShowFooter
    | KeyPressed Keyboard.KeyCode
    | KeyDowns Keyboard.KeyCode
    | TopBarMsg NewTopBar.Msg
    | PipelinePauseToggled Concourse.Pipeline (Result Http.Error ())
    | DragStart String Int
    | DragOver String Int
    | DragEnd
    | Tooltip String String
    | TooltipHd String String
    | TogglePipelinePaused Concourse.Pipeline
    | PipelineButtonHover (Maybe Concourse.Pipeline)
    | CliHover (Maybe Cli)

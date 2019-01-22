module Dashboard.Msgs exposing (Msg(..))

import Concourse.Cli as Cli
import Dashboard.Models as Models
import Keyboard
import Time
import Window


type Msg
    = ClockTick Time.Time
    | AutoRefresh Time.Time
    | ShowFooter
    | KeyPressed Keyboard.KeyCode
    | KeyDowns Keyboard.KeyCode
    | DragStart String Int
    | DragOver String Int
    | DragEnd
    | Tooltip String String
    | TooltipHd String String
    | TogglePipelinePaused Models.Pipeline
    | PipelineButtonHover (Maybe Models.Pipeline)
    | CliHover (Maybe Cli.Cli)
    | TopCliHover (Maybe Cli.Cli)
    | ResizeScreen Window.Size
    | LogIn
    | LogOut
    | FilterMsg String
    | FocusMsg
    | BlurMsg
    | SelectMsg Int
    | ToggleUserMenu
    | ShowSearchInput

module Pipeline.Msgs exposing (Msg(..))

import Concourse
import Keyboard
import NewTopBar.Msgs
import Time exposing (Time)


type Msg
    = AutoupdateVersionTicked Time
    | AutoupdateTimerTicked Time
    | HideLegendTimerTicked Time
    | ShowLegend
    | KeyPressed Keyboard.KeyCode
    | PipelineIdentifierFetched Concourse.PipelineIdentifier
    | ToggleGroup Concourse.PipelineGroup
    | SetGroups (List String)
    | FromTopBar NewTopBar.Msgs.Msg

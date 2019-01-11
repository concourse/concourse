module Pipeline.Msgs exposing (Msg(..))

import Concourse
import Http
import Json.Encode
import Keyboard
import Time exposing (Time)


type Msg
    = Noop
    | AutoupdateVersionTicked Time
    | AutoupdateTimerTicked Time
    | HideLegendTimerTicked Time
    | ShowLegend
    | KeyPressed Keyboard.KeyCode
    | PipelineIdentifierFetched Concourse.PipelineIdentifier
    | JobsFetched (Result Http.Error Json.Encode.Value)
    | ResourcesFetched (Result Http.Error Json.Encode.Value)
    | VersionFetched (Result Http.Error String)
    | PipelineFetched (Result Http.Error Concourse.Pipeline)
    | ToggleGroup Concourse.PipelineGroup
    | SetGroups (List String)

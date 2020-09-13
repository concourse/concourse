module Dashboard.Group.Models exposing
    ( Card(..)
    , Pipeline
    , cardName
    , cardTeamName
    )


type Card
    = PipelineCard Pipeline


cardName : Card -> String
cardName c =
    case c of
        PipelineCard p ->
            p.name


cardTeamName : Card -> String
cardTeamName c =
    case c of
        PipelineCard p ->
            p.teamName


type alias Pipeline =
    { id : Int
    , name : String
    , teamName : String
    , public : Bool
    , isToggleLoading : Bool
    , isVisibilityLoading : Bool
    , paused : Bool
    , archived : Bool
    , stale : Bool
    , jobsDisabled : Bool
    }

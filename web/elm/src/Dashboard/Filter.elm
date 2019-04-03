module Dashboard.Filter exposing (filterGroups)

import Concourse.PipelineStatus
    exposing
        ( PipelineStatus(..)
        , StatusDetails(..)
        , equal
        , isRunning
        )
import Dashboard.Group.Models exposing (Group, Pipeline)
import Parser
    exposing
        ( (|.)
        , (|=)
        , Parser
        , Step(..)
        , backtrackable
        , chompWhile
        , end
        , getChompedString
        , keyword
        , loop
        , map
        , oneOf
        , run
        , spaces
        , succeed
        , symbol
        )
import Simple.Fuzzy


type alias Filter =
    { negate : Bool
    , groupFilter : GroupFilter
    }


filterGroups : String -> List Group -> List Group
filterGroups query groups =
    filters query
        |> List.foldr runFilter groups


runFilter : Filter -> List Group -> List Group
runFilter f =
    let
        negater =
            if f.negate then
                not

            else
                identity
    in
    case f.groupFilter of
        Team teamName ->
            List.filter (.teamName >> Simple.Fuzzy.match teamName >> negater)

        Pipeline pf ->
            List.map
                (\g ->
                    { g
                        | pipelines =
                            g.pipelines
                                |> List.filter (pipelineFilter pf >> negater)
                    }
                )
                >> List.filter (.pipelines >> List.isEmpty >> not)


pipelineFilter : PipelineFilter -> Pipeline -> Bool
pipelineFilter pf =
    case pf of
        Status sf ->
            case sf of
                PipelineStatus ps ->
                    .status >> equal ps

                PipelineRunning ->
                    .status >> isRunning

        FuzzyName term ->
            .name >> Simple.Fuzzy.match term


filters : String -> List Filter
filters =
    run
        (loop [] <|
            \revFilters ->
                oneOf
                    [ end
                        |> map (\_ -> Done (List.reverse revFilters))
                    , filter
                        |> map (\f -> Loop (f :: revFilters))
                    ]
        )
        >> Result.withDefault []


filter : Parser Filter
filter =
    oneOf
        [ succeed (Filter True) |. spaces |. symbol "-" |= groupFilter |. spaces
        , succeed (Filter False) |. spaces |= groupFilter |. spaces
        ]


type GroupFilter
    = Team String
    | Pipeline PipelineFilter


type PipelineFilter
    = Status StatusFilter
    | FuzzyName String


groupFilter : Parser GroupFilter
groupFilter =
    oneOf
        [ backtrackable teamFilter
        , backtrackable statusFilter
        , succeed (FuzzyName >> Pipeline) |= parseWord
        ]


parseWord : Parser String
parseWord =
    getChompedString
        (chompWhile
            (\c -> c /= ' ' && c /= '\t' && c /= '\n' && c /= '\u{000D}')
        )


type StatusFilter
    = PipelineStatus PipelineStatus
    | PipelineRunning


teamFilter : Parser GroupFilter
teamFilter =
    succeed Team
        |. keyword "team"
        |. symbol ":"
        |. spaces
        |= parseWord


statusFilter : Parser GroupFilter
statusFilter =
    succeed (Status >> Pipeline)
        |. keyword "status"
        |. symbol ":"
        |. spaces
        |= pipelineStatus


pipelineStatus : Parser StatusFilter
pipelineStatus =
    oneOf
        [ map (\_ -> PipelineStatus PipelineStatusPaused) (keyword "paused")
        , map (\_ -> PipelineStatus <| PipelineStatusAborted Running)
            (keyword "aborted")
        , map (\_ -> PipelineStatus <| PipelineStatusErrored Running)
            (keyword "errored")
        , map (\_ -> PipelineStatus <| PipelineStatusFailed Running)
            (keyword "failed")
        , map (\_ -> PipelineStatus <| PipelineStatusPending False)
            (keyword "pending")
        , map (\_ -> PipelineStatus <| PipelineStatusSucceeded Running)
            (keyword "succeeded")
        , map (\_ -> PipelineRunning) (keyword "running")
        ]

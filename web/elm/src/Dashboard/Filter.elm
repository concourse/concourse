module Dashboard.Filter exposing (filterGroups)

import Concourse exposing (DatabaseID)
import Concourse.PipelineStatus
    exposing
        ( PipelineStatus(..)
        , StatusDetails(..)
        , equal
        , isRunning
        )
import Dashboard.Group.Models exposing (Group, Pipeline)
import Dashboard.Pipeline as Pipeline
import Dict exposing (Dict)
import Parser
    exposing
        ( (|.)
        , (|=)
        , Parser
        , Step(..)
        , backtrackable
        , chompUntilEndOr
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
import Routes
import Set exposing (Set)
import Simple.Fuzzy


type alias Filter =
    { negate : Bool
    , groupFilter : GroupFilter
    }


filterGroups :
    { pipelineJobs : Dict ( String, String ) (List Concourse.JobIdentifier)
    , jobs : Dict ( String, String, String ) Concourse.Job
    , query : String
    , teams : List Concourse.Team
    , pipelines : Dict String (List Pipeline)
    , dashboardView : Routes.DashboardView
    , favoritedPipelines : Set DatabaseID
    }
    -> List Group
filterGroups { pipelineJobs, jobs, query, teams, pipelines, dashboardView, favoritedPipelines } =
    let
        groupsToFilter =
            teams
                |> List.map (\t -> ( t.name, [] ))
                |> Dict.fromList
                |> Dict.union pipelines
                |> Dict.toList
                |> List.map
                    (\( t, p ) ->
                        { teamName = t
                        , pipelines = List.filter (prefilter dashboardView favoritedPipelines) p
                        }
                    )
    in
    parseFilters query |> List.foldr (runFilter jobs pipelineJobs) groupsToFilter


prefilter : Routes.DashboardView -> Set DatabaseID -> Pipeline -> Bool
prefilter view favoritedPipelines p =
    case view of
        Routes.ViewNonArchivedPipelines ->
            not p.archived || Set.member p.id favoritedPipelines

        _ ->
            True


runFilter : Dict ( String, String, String ) Concourse.Job -> Dict ( String, String ) (List Concourse.JobIdentifier) -> Filter -> List Group -> List Group
runFilter jobs existingJobs f =
    let
        negater =
            if f.negate then
                not

            else
                identity
    in
    case f.groupFilter of
        Team tf ->
            List.filter (.teamName >> stringMatches tf >> negater)

        Pipeline pf ->
            List.map
                (\g ->
                    { g
                        | pipelines =
                            g.pipelines
                                |> List.filter (pipelineFilter pf jobs existingJobs >> negater)
                    }
                )
                >> List.filter (.pipelines >> List.isEmpty >> not)


lookupJob : Dict ( String, String, String ) Concourse.Job -> Concourse.JobIdentifier -> Maybe Concourse.Job
lookupJob jobs jobId =
    jobs
        |> Dict.get ( jobId.teamName, jobId.pipelineName, jobId.jobName )


pipelineFilter : PipelineFilter -> Dict ( String, String, String ) Concourse.Job -> Dict ( String, String ) (List Concourse.JobIdentifier) -> Pipeline -> Bool
pipelineFilter pf jobs existingJobs pipeline =
    let
        jobsForPipeline =
            existingJobs
                |> Dict.get ( pipeline.teamName, pipeline.name )
                |> Maybe.withDefault []
                |> List.filterMap (lookupJob jobs)
    in
    case pf of
        Status sf ->
            case sf of
                PipelineStatus ps ->
                    pipeline |> Pipeline.pipelineStatus jobsForPipeline |> equal ps

                PipelineRunning ->
                    pipeline |> Pipeline.pipelineStatus jobsForPipeline |> isRunning

        Name nf ->
            pipeline.name |> stringMatches nf


parseFilters : String -> List Filter
parseFilters =
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
    succeed Filter
        |. spaces
        |= oneOf
            [ symbol "-" |> map (always True)
            , succeed False
            ]
        |= groupFilter
        |. spaces


type GroupFilter
    = Team StringFilter
    | Pipeline PipelineFilter


type PipelineFilter
    = Status StatusFilter
    | Name StringFilter


type StringFilter
    = Fuzzy String
    | Exact String
    | StartsWith String


stringMatches : StringFilter -> String -> Bool
stringMatches f =
    case f of
        Fuzzy term ->
            Simple.Fuzzy.match term

        Exact name ->
            (==) name

        StartsWith prefix ->
            String.startsWith prefix


groupFilter : Parser GroupFilter
groupFilter =
    oneOf
        [ backtrackable teamFilter
        , backtrackable statusFilter
        , succeed (Name >> Pipeline) |= parseString
        ]


parseString : Parser StringFilter
parseString =
    oneOf
        [ parseQuotedString
        , parseWord |> map Fuzzy
        ]


parseQuotedString : Parser StringFilter
parseQuotedString =
    getChompedString
        (symbol "\""
            |. chompUntilEndOr "\""
            |. oneOf [ symbol "\"", end ]
        )
        |> map
            (\s ->
                if String.endsWith "\"" s && s /= "\"" then
                    Exact <| String.slice 1 -1 s

                else
                    StartsWith <| String.dropLeft 1 s
            )


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
        |= parseString


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

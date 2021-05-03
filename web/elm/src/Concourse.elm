module Concourse exposing
    ( AuthSession
    , AuthToken
    , Build
    , BuildDuration
    , BuildId
    , BuildName
    , BuildPlan
    , BuildPrep
    , BuildPrepStatus(..)
    , BuildResources
    , BuildResourcesInput
    , BuildResourcesOutput
    , BuildStep(..)
    , CSRFToken
    , Cause
    , ClusterInfo
    , DatabaseID
    , HookedPlan
    , InstanceGroupIdentifier
    , InstanceVars
    , Job
    , JobBuildIdentifier
    , JobIdentifier
    , JobInput
    , JobName
    , JobOutput
    , JsonValue(..)
    , Metadata
    , MetadataField
    , Pipeline
    , PipelineGroup
    , PipelineGrouping(..)
    , PipelineIdentifier
    , PipelineName
    , Resource
    , ResourceIdentifier
    , Team
    , TeamName
    , User
    , Version
    , VersionedResource
    , VersionedResourceIdentifier
    , csrfTokenHeaderName
    , customDecoder
    , decodeAuthToken
    , decodeBuild
    , decodeBuildPlan
    , decodeBuildPlanResponse
    , decodeBuildPrep
    , decodeBuildResources
    , decodeCause
    , decodeInfo
    , decodeInstanceGroupId
    , decodeInstanceVars
    , decodeJob
    , decodeJsonValue
    , decodeMetadata
    , decodePipeline
    , decodeResource
    , decodeTeam
    , decodeUser
    , decodeVersion
    , decodeVersionedResource
    , emptyBuildResources
    , encodeBuild
    , encodeInstanceGroupId
    , encodeInstanceVars
    , encodeJob
    , encodeJsonValue
    , encodePipeline
    , encodeResource
    , encodeTeam
    , flattenJson
    , groupPipelinesWithinTeam
    , hyphenNotation
    , isInInstanceGroup
    , isInstanceGroup
    , mapBuildPlan
    , pipelineId
    , retrieveCSRFToken
    , toInstanceGroupId
    , toPipelineId
    , versionQuery
    )

import Array exposing (Array)
import Concourse.BuildStatus exposing (BuildStatus)
import Dict exposing (Dict)
import Json.Decode
import Json.Decode.Extra exposing (andMap)
import Json.Encode
import Json.Encode.Extra
import List.Extra
import Time



-- AuthToken


type alias AuthToken =
    String


type alias DatabaseID =
    Int


decodeAuthToken : Json.Decode.Decoder AuthToken
decodeAuthToken =
    customDecoder
        (Json.Decode.succeed (\a b -> ( a, b ))
            |> andMap (Json.Decode.field "type" Json.Decode.string)
            |> andMap (Json.Decode.field "value" Json.Decode.string)
        )
        authTokenFromTuple


authTokenFromTuple : ( String, String ) -> Result Json.Decode.Error AuthToken
authTokenFromTuple ( t, token ) =
    case t of
        "Bearer" ->
            Ok token

        _ ->
            Err <| Json.Decode.Failure "unknown token type" <| Json.Encode.string token



-- CSRF token


type alias CSRFToken =
    String


csrfTokenHeaderName : String
csrfTokenHeaderName =
    "X-Csrf-Token"


retrieveCSRFToken : Dict String String -> Result String CSRFToken
retrieveCSRFToken headers =
    Dict.get (String.toLower csrfTokenHeaderName) (keysToLower headers) |> Result.fromMaybe "error CSRFToken not found"


keysToLower : Dict String a -> Dict String a
keysToLower =
    Dict.fromList << List.map fstToLower << Dict.toList


fstToLower : ( String, a ) -> ( String, a )
fstToLower ( x, y ) =
    ( String.toLower x, y )


type alias AuthSession =
    { authToken : AuthToken
    , csrfToken : CSRFToken
    }



-- Build


type alias BuildId =
    Int


type alias BuildName =
    String


type alias JobBuildIdentifier =
    { teamName : TeamName
    , pipelineName : PipelineName
    , pipelineInstanceVars : InstanceVars
    , jobName : JobName
    , buildName : BuildName
    }


type alias Build =
    { id : BuildId
    , name : BuildName
    , teamName : TeamName
    , job : Maybe JobIdentifier
    , status : BuildStatus
    , duration : BuildDuration
    , reapTime : Maybe Time.Posix
    }


type alias BuildDuration =
    { startedAt : Maybe Time.Posix
    , finishedAt : Maybe Time.Posix
    }


encodeBuild : Build -> Json.Encode.Value
encodeBuild build =
    Json.Encode.object
        ([ ( "id", build.id |> Json.Encode.int ) |> Just
         , ( "name", build.name |> Json.Encode.string ) |> Just
         , ( "team_name", build.teamName |> Json.Encode.string ) |> Just
         , optionalField "pipeline_name" Json.Encode.string (build.job |> Maybe.map .pipelineName)
         , optionalField "pipeline_instance_vars" encodeInstanceVars (build.job |> Maybe.map .pipelineInstanceVars)
         , optionalField "job_name" Json.Encode.string (build.job |> Maybe.map .jobName)
         , ( "status", build.status |> Concourse.BuildStatus.encodeBuildStatus ) |> Just
         , optionalField "start_time" (secondsFromDate >> Json.Encode.int) build.duration.startedAt
         , optionalField "end_time" (secondsFromDate >> Json.Encode.int) build.duration.finishedAt
         , optionalField "reap_time" (secondsFromDate >> Json.Encode.int) build.reapTime
         ]
            |> List.filterMap identity
        )


encodeMaybeBuild : Maybe Build -> Json.Encode.Value
encodeMaybeBuild maybeBuild =
    case maybeBuild of
        Nothing ->
            Json.Encode.null

        Just build ->
            encodeBuild build


decodeBuild : Json.Decode.Decoder Build
decodeBuild =
    Json.Decode.succeed Build
        |> andMap (Json.Decode.field "id" Json.Decode.int)
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "team_name" Json.Decode.string)
        |> andMap
            (Json.Decode.maybe
                (Json.Decode.succeed JobIdentifier
                    |> andMap (Json.Decode.field "team_name" Json.Decode.string)
                    |> andMap (Json.Decode.field "pipeline_name" Json.Decode.string)
                    |> andMap (defaultTo Dict.empty <| Json.Decode.field "pipeline_instance_vars" <| decodeInstanceVars)
                    |> andMap (Json.Decode.field "job_name" Json.Decode.string)
                )
            )
        |> andMap (Json.Decode.field "status" Concourse.BuildStatus.decodeBuildStatus)
        |> andMap
            (Json.Decode.succeed BuildDuration
                |> andMap (Json.Decode.maybe (Json.Decode.field "start_time" (Json.Decode.map dateFromSeconds Json.Decode.int)))
                |> andMap (Json.Decode.maybe (Json.Decode.field "end_time" (Json.Decode.map dateFromSeconds Json.Decode.int)))
            )
        |> andMap (Json.Decode.maybe (Json.Decode.field "reap_time" (Json.Decode.map dateFromSeconds Json.Decode.int)))



-- BuildPrep


type alias BuildPrep =
    { pausedPipeline : BuildPrepStatus
    , pausedJob : BuildPrepStatus
    , maxRunningBuilds : BuildPrepStatus
    , inputs : Dict String BuildPrepStatus
    , inputsSatisfied : BuildPrepStatus
    , missingInputReasons : Dict String String
    }


type BuildPrepStatus
    = BuildPrepStatusUnknown
    | BuildPrepStatusBlocking
    | BuildPrepStatusNotBlocking


decodeBuildPrep : Json.Decode.Decoder BuildPrep
decodeBuildPrep =
    Json.Decode.succeed BuildPrep
        |> andMap (Json.Decode.field "paused_pipeline" decodeBuildPrepStatus)
        |> andMap (Json.Decode.field "paused_job" decodeBuildPrepStatus)
        |> andMap (Json.Decode.field "max_running_builds" decodeBuildPrepStatus)
        |> andMap (Json.Decode.field "inputs" <| Json.Decode.dict decodeBuildPrepStatus)
        |> andMap (Json.Decode.field "inputs_satisfied" decodeBuildPrepStatus)
        |> andMap (defaultTo Dict.empty <| Json.Decode.field "missing_input_reasons" <| Json.Decode.dict Json.Decode.string)


decodeBuildPrepStatus : Json.Decode.Decoder BuildPrepStatus
decodeBuildPrepStatus =
    customDecoder Json.Decode.string <|
        \status ->
            case status of
                "unknown" ->
                    Ok BuildPrepStatusUnknown

                "blocking" ->
                    Ok BuildPrepStatusBlocking

                "not_blocking" ->
                    Ok BuildPrepStatusNotBlocking

                unknown ->
                    Err <| Json.Decode.Failure "unknown build preparation status" <| Json.Encode.string unknown



-- BuildResources


type alias BuildResources =
    { inputs : List BuildResourcesInput
    , outputs : List BuildResourcesOutput
    }


type alias BuildResourcesInput =
    { name : String
    , version : Version
    , firstOccurrence : Bool
    }


type alias BuildResourcesOutput =
    { name : String
    , version : Version
    }


emptyBuildResources : BuildResources
emptyBuildResources =
    { inputs = []
    , outputs = []
    }


decodeBuildResources : Json.Decode.Decoder BuildResources
decodeBuildResources =
    Json.Decode.succeed BuildResources
        |> andMap (Json.Decode.field "inputs" <| Json.Decode.list decodeResourcesInput)
        |> andMap (Json.Decode.field "outputs" <| Json.Decode.list decodeResourcesOutput)


decodeResourcesInput : Json.Decode.Decoder BuildResourcesInput
decodeResourcesInput =
    Json.Decode.succeed BuildResourcesInput
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "version" decodeVersion)
        |> andMap (Json.Decode.field "first_occurrence" Json.Decode.bool)


decodeResourcesOutput : Json.Decode.Decoder BuildResourcesOutput
decodeResourcesOutput =
    Json.Decode.succeed BuildResourcesOutput
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "version" <| Json.Decode.dict Json.Decode.string)



-- BuildPlan


type alias BuildPlan =
    { id : String
    , step : BuildStep
    }


mapBuildPlan : (BuildPlan -> a) -> BuildPlan -> List a
mapBuildPlan fn plan =
    fn plan
        :: (case plan.step of
                BuildStepTask _ ->
                    []

                BuildStepSetPipeline _ _ ->
                    []

                BuildStepLoadVar _ ->
                    []

                BuildStepArtifactInput _ ->
                    []

                BuildStepPut _ _ ->
                    []

                BuildStepCheck _ ->
                    []

                BuildStepGet _ _ _ ->
                    []

                BuildStepArtifactOutput _ ->
                    []

                BuildStepInParallel plans ->
                    List.concatMap (mapBuildPlan fn) (Array.toList plans)

                BuildStepAcross { steps } ->
                    List.concatMap (mapBuildPlan fn)
                        (steps |> List.map Tuple.second)

                BuildStepDo plans ->
                    List.concatMap (mapBuildPlan fn) (Array.toList plans)

                BuildStepOnSuccess { step, hook } ->
                    mapBuildPlan fn step ++ mapBuildPlan fn hook

                BuildStepOnFailure { step, hook } ->
                    mapBuildPlan fn step ++ mapBuildPlan fn hook

                BuildStepOnAbort { step, hook } ->
                    mapBuildPlan fn step ++ mapBuildPlan fn hook

                BuildStepOnError { step, hook } ->
                    mapBuildPlan fn step ++ mapBuildPlan fn hook

                BuildStepEnsure { step, hook } ->
                    mapBuildPlan fn step ++ mapBuildPlan fn hook

                BuildStepTry step ->
                    mapBuildPlan fn step

                BuildStepRetry plans ->
                    List.concatMap (mapBuildPlan fn) (Array.toList plans)

                BuildStepTimeout step ->
                    mapBuildPlan fn step
           )


type alias StepName =
    String


type alias ResourceName =
    String


type BuildStep
    = BuildStepTask StepName
    | BuildStepSetPipeline StepName InstanceVars
    | BuildStepLoadVar StepName
    | BuildStepArtifactInput StepName
    | BuildStepCheck StepName
    | BuildStepGet StepName (Maybe ResourceName) (Maybe Version)
    | BuildStepArtifactOutput StepName
    | BuildStepPut StepName (Maybe ResourceName)
    | BuildStepInParallel (Array BuildPlan)
    | BuildStepAcross AcrossPlan
    | BuildStepDo (Array BuildPlan)
    | BuildStepOnSuccess HookedPlan
    | BuildStepOnFailure HookedPlan
    | BuildStepOnAbort HookedPlan
    | BuildStepOnError HookedPlan
    | BuildStepEnsure HookedPlan
    | BuildStepTry BuildPlan
    | BuildStepRetry (Array BuildPlan)
    | BuildStepTimeout BuildPlan


type alias HookedPlan =
    { step : BuildPlan
    , hook : BuildPlan
    }


type JsonValue
    = JsonString String
    | JsonNumber Float
    | JsonObject (List ( String, JsonValue ))
    | JsonRaw Json.Decode.Value


decodeJsonValue : Json.Decode.Decoder JsonValue
decodeJsonValue =
    Json.Decode.oneOf
        [ Json.Decode.keyValuePairs (Json.Decode.lazy (\_ -> decodeJsonValue))
            |> Json.Decode.map (List.sortBy Tuple.first)
            |> Json.Decode.map JsonObject
        , decodeSimpleJsonValue
        ]


decodeSimpleJsonValue : Json.Decode.Decoder JsonValue
decodeSimpleJsonValue =
    Json.Decode.oneOf
        [ Json.Decode.string |> Json.Decode.map JsonString
        , Json.Decode.float |> Json.Decode.map JsonNumber
        , Json.Decode.value |> Json.Decode.map JsonRaw
        ]


encodeJsonValue : JsonValue -> Json.Encode.Value
encodeJsonValue v =
    case v of
        JsonString s ->
            Json.Encode.string s

        JsonNumber f ->
            Json.Encode.float f

        JsonObject kvs ->
            encodeJsonObject kvs

        JsonRaw raw ->
            raw


encodeJsonObject : List ( String, JsonValue ) -> Json.Encode.Value
encodeJsonObject =
    List.sortBy Tuple.first
        >> List.map (Tuple.mapSecond encodeJsonValue)
        >> Json.Encode.object


flattenJson : String -> JsonValue -> List ( String, String )
flattenJson key val =
    case val of
        JsonString s ->
            [ ( key, s ) ]

        JsonNumber n ->
            [ ( key, String.fromFloat n ) ]

        JsonRaw v ->
            [ ( key, Json.Encode.encode 0 v ) ]

        JsonObject o ->
            List.concatMap
                (\( k, v ) ->
                    let
                        subKey =
                            key ++ "." ++ k
                    in
                    flattenJson subKey v
                )
                o


hyphenNotation : Dict String JsonValue -> String
hyphenNotation vars =
    if Dict.isEmpty vars then
        "{}"

    else
        vars
            |> Dict.toList
            |> List.concatMap (\( k, v ) -> flattenJson k v)
            |> List.map Tuple.second
            |> String.join "-"


type PipelineGrouping pipeline
    = RegularPipeline pipeline
    | InstanceGroup pipeline (List pipeline)


groupPipelinesWithinTeam :
    List { p | name : String, instanceVars : InstanceVars }
    -> List (PipelineGrouping { p | name : String, instanceVars : InstanceVars })
groupPipelinesWithinTeam =
    List.Extra.gatherEqualsBy .name
        >> List.map
            (\( p, ps ) ->
                if isInstanceGroup (p :: ps) then
                    InstanceGroup p ps

                else
                    RegularPipeline p
            )


isInstanceGroup : List { p | instanceVars : InstanceVars } -> Bool
isInstanceGroup pipelines =
    case pipelines of
        p :: ps ->
            not (List.isEmpty ps && Dict.isEmpty p.instanceVars)

        _ ->
            False


isInInstanceGroup :
    List { a | id : DatabaseID, name : String, teamName : String, instanceVars : InstanceVars }
    -> { b | id : DatabaseID, name : String, teamName : String, instanceVars : InstanceVars }
    -> Bool
isInInstanceGroup allPipelines p =
    not (Dict.isEmpty p.instanceVars)
        || List.any
            (\p2 ->
                (p.name == p2.name)
                    && (p.teamName == p2.teamName)
                    && (p.id /= p2.id)
            )
            allPipelines


type alias AcrossPlan =
    { vars : List String
    , steps : List ( List JsonValue, BuildPlan )
    }


decodeBuildPlanResponse : Json.Decode.Decoder BuildPlan
decodeBuildPlanResponse =
    Json.Decode.at [ "plan" ] decodeBuildPlan


decodeBuildPlan : Json.Decode.Decoder BuildPlan
decodeBuildPlan =
    Json.Decode.succeed BuildPlan
        |> andMap (Json.Decode.field "id" Json.Decode.string)
        |> andMap
            (Json.Decode.oneOf
                -- buckle up
                [ Json.Decode.field "task" <|
                    lazy (\_ -> decodeBuildStepTask)
                , Json.Decode.field "check" <|
                    lazy (\_ -> decodeBuildStepCheck)
                , Json.Decode.field "get" <|
                    lazy (\_ -> decodeBuildStepGet)
                , Json.Decode.field "artifact_input" <|
                    lazy (\_ -> decodeBuildStepArtifactInput)
                , Json.Decode.field "put" <|
                    lazy (\_ -> decodeBuildStepPut)
                , Json.Decode.field "artifact_output" <|
                    lazy (\_ -> decodeBuildStepArtifactOutput)
                , Json.Decode.field "dependent_get" <|
                    lazy (\_ -> decodeBuildStepGet)
                , Json.Decode.field "in_parallel" <|
                    lazy (\_ -> decodeBuildStepInParallel)
                , Json.Decode.field "do" <|
                    lazy (\_ -> decodeBuildStepDo)
                , Json.Decode.field "on_success" <|
                    lazy (\_ -> decodeBuildStepOnSuccess)
                , Json.Decode.field "on_failure" <|
                    lazy (\_ -> decodeBuildStepOnFailure)
                , Json.Decode.field "on_abort" <|
                    lazy (\_ -> decodeBuildStepOnAbort)
                , Json.Decode.field "on_error" <|
                    lazy (\_ -> decodeBuildStepOnError)
                , Json.Decode.field "ensure" <|
                    lazy (\_ -> decodeBuildStepEnsure)
                , Json.Decode.field "try" <|
                    lazy (\_ -> decodeBuildStepTry)
                , Json.Decode.field "retry" <|
                    lazy (\_ -> decodeBuildStepRetry)
                , Json.Decode.field "timeout" <|
                    lazy (\_ -> decodeBuildStepTimeout)
                , Json.Decode.field "set_pipeline" <|
                    lazy (\_ -> decodeBuildSetPipeline)
                , Json.Decode.field "load_var" <|
                    lazy (\_ -> decodeBuildStepLoadVar)
                , Json.Decode.field "across" <|
                    lazy (\_ -> decodeBuildStepAcross)
                ]
            )


decodeBuildStepTask : Json.Decode.Decoder BuildStep
decodeBuildStepTask =
    Json.Decode.succeed BuildStepTask
        |> andMap (Json.Decode.field "name" Json.Decode.string)


decodeBuildStepArtifactInput : Json.Decode.Decoder BuildStep
decodeBuildStepArtifactInput =
    Json.Decode.succeed BuildStepArtifactInput
        |> andMap (Json.Decode.field "name" Json.Decode.string)


decodeBuildStepGet : Json.Decode.Decoder BuildStep
decodeBuildStepGet =
    Json.Decode.succeed BuildStepGet
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.maybe <| Json.Decode.field "resource" Json.Decode.string)
        |> andMap (Json.Decode.maybe <| Json.Decode.field "version" decodeVersion)


decodeBuildStepCheck : Json.Decode.Decoder BuildStep
decodeBuildStepCheck =
    Json.Decode.succeed BuildStepCheck
        |> andMap (Json.Decode.field "name" Json.Decode.string)


decodeBuildStepArtifactOutput : Json.Decode.Decoder BuildStep
decodeBuildStepArtifactOutput =
    Json.Decode.succeed BuildStepArtifactOutput
        |> andMap (Json.Decode.field "name" Json.Decode.string)


decodeBuildStepPut : Json.Decode.Decoder BuildStep
decodeBuildStepPut =
    Json.Decode.succeed BuildStepPut
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.maybe <| Json.Decode.field "resource" Json.Decode.string)


decodeBuildStepInParallel : Json.Decode.Decoder BuildStep
decodeBuildStepInParallel =
    Json.Decode.succeed BuildStepInParallel
        |> andMap (Json.Decode.field "steps" <| Json.Decode.array (lazy (\_ -> decodeBuildPlan)))


decodeBuildStepDo : Json.Decode.Decoder BuildStep
decodeBuildStepDo =
    Json.Decode.succeed BuildStepDo
        |> andMap (Json.Decode.array (lazy (\_ -> decodeBuildPlan)))


decodeBuildStepOnSuccess : Json.Decode.Decoder BuildStep
decodeBuildStepOnSuccess =
    Json.Decode.map BuildStepOnSuccess
        (Json.Decode.succeed HookedPlan
            |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan))
            |> andMap (Json.Decode.field "on_success" <| lazy (\_ -> decodeBuildPlan))
        )


decodeBuildStepOnFailure : Json.Decode.Decoder BuildStep
decodeBuildStepOnFailure =
    Json.Decode.map BuildStepOnFailure
        (Json.Decode.succeed HookedPlan
            |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan))
            |> andMap (Json.Decode.field "on_failure" <| lazy (\_ -> decodeBuildPlan))
        )


decodeBuildStepOnAbort : Json.Decode.Decoder BuildStep
decodeBuildStepOnAbort =
    Json.Decode.map BuildStepOnAbort
        (Json.Decode.succeed HookedPlan
            |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan))
            |> andMap (Json.Decode.field "on_abort" <| lazy (\_ -> decodeBuildPlan))
        )


decodeBuildStepOnError : Json.Decode.Decoder BuildStep
decodeBuildStepOnError =
    Json.Decode.map BuildStepOnError
        (Json.Decode.succeed HookedPlan
            |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan))
            |> andMap (Json.Decode.field "on_error" <| lazy (\_ -> decodeBuildPlan))
        )


decodeBuildStepEnsure : Json.Decode.Decoder BuildStep
decodeBuildStepEnsure =
    Json.Decode.map BuildStepEnsure
        (Json.Decode.succeed HookedPlan
            |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan))
            |> andMap (Json.Decode.field "ensure" <| lazy (\_ -> decodeBuildPlan))
        )


decodeBuildStepTry : Json.Decode.Decoder BuildStep
decodeBuildStepTry =
    Json.Decode.succeed BuildStepTry
        |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan))


decodeBuildStepRetry : Json.Decode.Decoder BuildStep
decodeBuildStepRetry =
    Json.Decode.succeed BuildStepRetry
        |> andMap (Json.Decode.array (lazy (\_ -> decodeBuildPlan)))


decodeBuildStepTimeout : Json.Decode.Decoder BuildStep
decodeBuildStepTimeout =
    Json.Decode.succeed BuildStepTimeout
        |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan))


decodeBuildSetPipeline : Json.Decode.Decoder BuildStep
decodeBuildSetPipeline =
    Json.Decode.succeed BuildStepSetPipeline
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (defaultTo Dict.empty <| Json.Decode.field "instance_vars" decodeInstanceVars)


decodeBuildStepLoadVar : Json.Decode.Decoder BuildStep
decodeBuildStepLoadVar =
    Json.Decode.succeed BuildStepLoadVar
        |> andMap (Json.Decode.field "name" Json.Decode.string)


decodeBuildStepAcross : Json.Decode.Decoder BuildStep
decodeBuildStepAcross =
    Json.Decode.map BuildStepAcross
        (Json.Decode.succeed AcrossPlan
            |> andMap
                (Json.Decode.field "vars" <|
                    Json.Decode.list <|
                        Json.Decode.field "name" Json.Decode.string
                )
            |> andMap
                (Json.Decode.field "steps" <|
                    Json.Decode.list <|
                        Json.Decode.map2 Tuple.pair
                            (Json.Decode.field "values" <| Json.Decode.list decodeJsonValue)
                            (Json.Decode.field "step" decodeBuildPlan)
                )
        )



-- Info


type alias ClusterInfo =
    { version : String
    , clusterName : String
    }


decodeInfo : Json.Decode.Decoder ClusterInfo
decodeInfo =
    Json.Decode.succeed ClusterInfo
        |> andMap (Json.Decode.field "version" Json.Decode.string)
        |> andMap (defaultTo "" <| Json.Decode.field "cluster_name" Json.Decode.string)



-- Job


type alias JobName =
    String


type alias JobIdentifier =
    { teamName : TeamName
    , pipelineName : PipelineName
    , pipelineInstanceVars : InstanceVars
    , jobName : JobName
    }


type alias Job =
    { name : JobName
    , pipelineId : DatabaseID
    , pipelineName : PipelineName
    , pipelineInstanceVars : InstanceVars
    , teamName : TeamName
    , nextBuild : Maybe Build
    , finishedBuild : Maybe Build
    , transitionBuild : Maybe Build
    , paused : Bool
    , disableManualTrigger : Bool
    , inputs : List JobInput
    , outputs : List JobOutput
    , groups : List String
    }


type alias JobInput =
    { name : String
    , resource : String
    , passed : List String
    , trigger : Bool
    }


type alias JobOutput =
    { name : String
    , resource : String
    }


encodeJob : Job -> Json.Encode.Value
encodeJob job =
    Json.Encode.object
        [ ( "name", job.name |> Json.Encode.string )
        , ( "pipeline_id", job.pipelineId |> Json.Encode.int )
        , ( "pipeline_name", job.pipelineName |> Json.Encode.string )
        , ( "pipeline_instance_vars", job.pipelineInstanceVars |> encodeInstanceVars )
        , ( "team_name", job.teamName |> Json.Encode.string )
        , ( "next_build", job.nextBuild |> encodeMaybeBuild )
        , ( "finished_build", job.finishedBuild |> encodeMaybeBuild )
        , ( "transition_build", job.finishedBuild |> encodeMaybeBuild )
        , ( "paused", job.paused |> Json.Encode.bool )
        , ( "disable_manual_trigger", job.disableManualTrigger |> Json.Encode.bool )
        , ( "inputs", job.inputs |> Json.Encode.list encodeJobInput )
        , ( "outputs", job.outputs |> Json.Encode.list encodeJobOutput )
        , ( "groups", job.groups |> Json.Encode.list Json.Encode.string )
        ]


decodeJob : Json.Decode.Decoder Job
decodeJob =
    Json.Decode.succeed Job
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "pipeline_id" Json.Decode.int)
        |> andMap (Json.Decode.field "pipeline_name" Json.Decode.string)
        |> andMap (defaultTo Dict.empty <| Json.Decode.field "pipeline_instance_vars" <| decodeInstanceVars)
        |> andMap (Json.Decode.field "team_name" Json.Decode.string)
        |> andMap (Json.Decode.maybe (Json.Decode.field "next_build" decodeBuild))
        |> andMap (Json.Decode.maybe (Json.Decode.field "finished_build" decodeBuild))
        |> andMap (Json.Decode.maybe (Json.Decode.field "transition_build" decodeBuild))
        |> andMap (defaultTo False <| Json.Decode.field "paused" Json.Decode.bool)
        |> andMap (defaultTo False <| Json.Decode.field "disable_manual_trigger" Json.Decode.bool)
        |> andMap (defaultTo [] <| Json.Decode.field "inputs" <| Json.Decode.list decodeJobInput)
        |> andMap (defaultTo [] <| Json.Decode.field "outputs" <| Json.Decode.list decodeJobOutput)
        |> andMap (defaultTo [] <| Json.Decode.field "groups" <| Json.Decode.list Json.Decode.string)


encodeJobInput : JobInput -> Json.Encode.Value
encodeJobInput jobInput =
    Json.Encode.object
        [ ( "name", jobInput.name |> Json.Encode.string )
        , ( "resource", jobInput.resource |> Json.Encode.string )
        , ( "passed", jobInput.passed |> Json.Encode.list Json.Encode.string )
        , ( "trigger", jobInput.trigger |> Json.Encode.bool )
        ]


decodeJobInput : Json.Decode.Decoder JobInput
decodeJobInput =
    Json.Decode.succeed JobInput
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "resource" Json.Decode.string)
        |> andMap (defaultTo [] <| Json.Decode.field "passed" <| Json.Decode.list Json.Decode.string)
        |> andMap (defaultTo False <| Json.Decode.field "trigger" Json.Decode.bool)


encodeJobOutput : JobOutput -> Json.Encode.Value
encodeJobOutput jobOutput =
    Json.Encode.object
        [ ( "name", jobOutput.name |> Json.Encode.string )
        , ( "resource", jobOutput.resource |> Json.Encode.string )
        ]


decodeJobOutput : Json.Decode.Decoder JobOutput
decodeJobOutput =
    Json.Decode.succeed JobOutput
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "resource" Json.Decode.string)



-- Pipeline


type alias PipelineName =
    String


type alias InstanceVars =
    Dict String JsonValue


encodeInstanceVars : InstanceVars -> Json.Encode.Value
encodeInstanceVars =
    Dict.toList >> encodeJsonObject


decodeInstanceVars : Json.Decode.Decoder InstanceVars
decodeInstanceVars =
    Json.Decode.dict decodeJsonValue


type alias PipelineIdentifier =
    { teamName : TeamName
    , pipelineName : PipelineName
    , pipelineInstanceVars : InstanceVars
    }


pipelineId : { r | teamName : TeamName, pipelineName : PipelineName, pipelineInstanceVars : InstanceVars } -> PipelineIdentifier
pipelineId { teamName, pipelineName, pipelineInstanceVars } =
    { teamName = teamName
    , pipelineName = pipelineName
    , pipelineInstanceVars = pipelineInstanceVars
    }


toPipelineId : { r | teamName : TeamName, name : PipelineName, instanceVars : InstanceVars } -> PipelineIdentifier
toPipelineId p =
    { teamName = p.teamName
    , pipelineName = p.name
    , pipelineInstanceVars = p.instanceVars
    }


type alias Pipeline =
    { id : Int
    , name : PipelineName
    , instanceVars : InstanceVars
    , paused : Bool
    , archived : Bool
    , public : Bool
    , teamName : TeamName
    , groups : List PipelineGroup
    , backgroundImage : Maybe String
    }


type alias PipelineGroup =
    { name : String
    , jobs : List String
    , resources : List String
    }


encodePipeline : Pipeline -> Json.Encode.Value
encodePipeline pipeline =
    Json.Encode.object
        [ ( "id", pipeline.id |> Json.Encode.int )
        , ( "name", pipeline.name |> Json.Encode.string )
        , ( "instance_vars", pipeline.instanceVars |> encodeInstanceVars )
        , ( "paused", pipeline.paused |> Json.Encode.bool )
        , ( "archived", pipeline.archived |> Json.Encode.bool )
        , ( "public", pipeline.public |> Json.Encode.bool )
        , ( "team_name", pipeline.teamName |> Json.Encode.string )
        , ( "groups", pipeline.groups |> Json.Encode.list encodePipelineGroup )
        , ( "display", Json.Encode.object [ ( "background_image", pipeline.backgroundImage |> Json.Encode.Extra.maybe Json.Encode.string ) ] )
        ]


decodePipeline : Json.Decode.Decoder Pipeline
decodePipeline =
    Json.Decode.succeed Pipeline
        |> andMap (Json.Decode.field "id" Json.Decode.int)
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (defaultTo Dict.empty <| Json.Decode.field "instance_vars" <| decodeInstanceVars)
        |> andMap (Json.Decode.field "paused" Json.Decode.bool)
        |> andMap (Json.Decode.field "archived" Json.Decode.bool)
        |> andMap (Json.Decode.field "public" Json.Decode.bool)
        |> andMap (Json.Decode.field "team_name" Json.Decode.string)
        |> andMap (defaultTo [] <| Json.Decode.field "groups" (Json.Decode.list decodePipelineGroup))
        |> andMap (Json.Decode.maybe (Json.Decode.at [ "display", "background_image" ] Json.Decode.string))


encodePipelineGroup : PipelineGroup -> Json.Encode.Value
encodePipelineGroup pipelineGroup =
    Json.Encode.object
        [ ( "name", pipelineGroup.name |> Json.Encode.string )
        , ( "jobs", pipelineGroup.jobs |> Json.Encode.list Json.Encode.string )
        , ( "resources", pipelineGroup.resources |> Json.Encode.list Json.Encode.string )
        ]


decodePipelineGroup : Json.Decode.Decoder PipelineGroup
decodePipelineGroup =
    Json.Decode.succeed PipelineGroup
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (defaultTo [] <| Json.Decode.field "jobs" <| Json.Decode.list Json.Decode.string)
        |> andMap (defaultTo [] <| Json.Decode.field "resources" <| Json.Decode.list Json.Decode.string)


type alias InstanceGroupIdentifier =
    { teamName : TeamName
    , name : PipelineName
    }


toInstanceGroupId : { p | teamName : TeamName, name : PipelineName } -> InstanceGroupIdentifier
toInstanceGroupId { teamName, name } =
    { teamName = teamName, name = name }


encodeInstanceGroupId : InstanceGroupIdentifier -> Json.Encode.Value
encodeInstanceGroupId { teamName, name } =
    Json.Encode.object
        [ ( "team_name", Json.Encode.string teamName )
        , ( "name", Json.Encode.string name )
        ]


decodeInstanceGroupId : Json.Decode.Decoder InstanceGroupIdentifier
decodeInstanceGroupId =
    Json.Decode.succeed InstanceGroupIdentifier
        |> andMap (Json.Decode.field "team_name" Json.Decode.string)
        |> andMap (Json.Decode.field "name" Json.Decode.string)



-- Resource


type alias Resource =
    { teamName : String
    , pipelineId : DatabaseID
    , pipelineName : String
    , pipelineInstanceVars : InstanceVars
    , name : String
    , icon : Maybe String
    , lastChecked : Maybe Time.Posix
    , pinnedVersion : Maybe Version
    , pinnedInConfig : Bool
    , pinComment : Maybe String
    , build : Maybe Build
    }


type alias ResourceIdentifier =
    { teamName : String
    , pipelineName : String
    , pipelineInstanceVars : InstanceVars
    , resourceName : String
    }


type alias VersionedResource =
    { id : Int
    , version : Version
    , metadata : Metadata
    , enabled : Bool
    }


type alias VersionedResourceIdentifier =
    { teamName : String
    , pipelineName : String
    , pipelineInstanceVars : InstanceVars
    , resourceName : String
    , versionID : Int
    }


decodeResource : Json.Decode.Decoder Resource
decodeResource =
    Json.Decode.succeed Resource
        |> andMap (Json.Decode.field "team_name" Json.Decode.string)
        |> andMap (Json.Decode.field "pipeline_id" Json.Decode.int)
        |> andMap (Json.Decode.field "pipeline_name" Json.Decode.string)
        |> andMap (defaultTo Dict.empty <| Json.Decode.field "pipeline_instance_vars" <| decodeInstanceVars)
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.maybe (Json.Decode.field "icon" Json.Decode.string))
        |> andMap (Json.Decode.maybe (Json.Decode.field "last_checked" (Json.Decode.map dateFromSeconds Json.Decode.int)))
        |> andMap (Json.Decode.maybe (Json.Decode.field "pinned_version" decodeVersion))
        |> andMap (defaultTo False <| Json.Decode.field "pinned_in_config" Json.Decode.bool)
        |> andMap (Json.Decode.maybe (Json.Decode.field "pin_comment" Json.Decode.string))
        |> andMap (Json.Decode.maybe (Json.Decode.field "build" decodeBuild))


encodeResource : Resource -> Json.Encode.Value
encodeResource r =
    Json.Encode.object
        ([ ( "team_name", r.teamName |> Json.Encode.string ) |> Just
         , ( "pipeline_id", r.pipelineId |> Json.Encode.int ) |> Just
         , ( "pipeline_name", r.pipelineName |> Json.Encode.string ) |> Just
         , ( "pipeline_instance_vars", r.pipelineInstanceVars |> encodeInstanceVars ) |> Just
         , ( "name", r.name |> Json.Encode.string ) |> Just
         , optionalField "icon" Json.Encode.string r.icon
         , optionalField "last_checked" (secondsFromDate >> Json.Encode.int) r.lastChecked
         , optionalField "pinned_version" encodeVersion r.pinnedVersion
         , ( "pinned_in_config", r.pinnedInConfig |> Json.Encode.bool ) |> Just
         , optionalField "pin_comment" Json.Encode.string r.pinComment
         , ( "build", r.build |> encodeMaybeBuild ) |> Just
         ]
            |> List.filterMap identity
        )


decodeVersionedResource : Json.Decode.Decoder VersionedResource
decodeVersionedResource =
    Json.Decode.succeed VersionedResource
        |> andMap (Json.Decode.field "id" Json.Decode.int)
        |> andMap (Json.Decode.field "version" decodeVersion)
        |> andMap (defaultTo [] (Json.Decode.field "metadata" decodeMetadata))
        |> andMap (Json.Decode.field "enabled" Json.Decode.bool)



-- Version


type alias Version =
    Dict String String


decodeVersion : Json.Decode.Decoder Version
decodeVersion =
    Json.Decode.dict Json.Decode.string


encodeVersion : Version -> Json.Encode.Value
encodeVersion =
    Json.Encode.dict identity Json.Encode.string


versionQuery : Version -> List String
versionQuery v =
    List.map (\kv -> Tuple.first kv ++ ":" ++ Tuple.second kv) <| Dict.toList v



-- Metadata


type alias Metadata =
    List MetadataField


type alias MetadataField =
    { name : String
    , value : String
    }


decodeMetadata : Json.Decode.Decoder (List MetadataField)
decodeMetadata =
    Json.Decode.list decodeMetadataField


decodeMetadataField : Json.Decode.Decoder MetadataField
decodeMetadataField =
    Json.Decode.succeed MetadataField
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "value" Json.Decode.string)



-- Team


type alias TeamName =
    String


type alias Team =
    { id : Int
    , name : TeamName
    }


encodeTeam : Team -> Json.Encode.Value
encodeTeam team =
    Json.Encode.object
        [ ( "id", team.id |> Json.Encode.int )
        , ( "name", team.name |> Json.Encode.string )
        ]


decodeTeam : Json.Decode.Decoder Team
decodeTeam =
    Json.Decode.succeed Team
        |> andMap (Json.Decode.field "id" Json.Decode.int)
        |> andMap (Json.Decode.field "name" Json.Decode.string)



-- User


type alias User =
    { id : String
    , userName : String
    , name : String
    , email : String
    , isAdmin : Bool
    , teams : Dict String (List String)
    , displayUserId : String
    }


decodeUser : Json.Decode.Decoder User
decodeUser =
    Json.Decode.succeed User
        |> andMap (Json.Decode.field "user_id" Json.Decode.string)
        |> andMap (Json.Decode.field "user_name" Json.Decode.string)
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "email" Json.Decode.string)
        |> andMap (Json.Decode.field "is_admin" Json.Decode.bool)
        |> andMap (Json.Decode.field "teams" (Json.Decode.dict (Json.Decode.list Json.Decode.string)))
        |> andMap (Json.Decode.field "display_user_id" Json.Decode.string)


-- Cause


type alias Cause =
    { versionedResourceID : Int
    , buildID : Int
    }


decodeCause : Json.Decode.Decoder Cause
decodeCause =
    Json.Decode.succeed Cause
        |> andMap (Json.Decode.field "versioned_resource_id" Json.Decode.int)
        |> andMap (Json.Decode.field "build_id" Json.Decode.int)



-- Helpers


dateFromSeconds : Int -> Time.Posix
dateFromSeconds =
    Time.millisToPosix << (*) 1000


secondsFromDate : Time.Posix -> Int
secondsFromDate =
    Time.posixToMillis >> (\m -> m // 1000)


lazy : (() -> Json.Decode.Decoder a) -> Json.Decode.Decoder a
lazy thunk =
    customDecoder Json.Decode.value
        (\js -> Json.Decode.decodeValue (thunk ()) js)


defaultTo : a -> Json.Decode.Decoder a -> Json.Decode.Decoder a
defaultTo default =
    Json.Decode.map (Maybe.withDefault default) << Json.Decode.maybe


customDecoder : Json.Decode.Decoder b -> (b -> Result Json.Decode.Error a) -> Json.Decode.Decoder a
customDecoder decoder toResult =
    Json.Decode.andThen
        (\a ->
            case toResult a of
                Ok b ->
                    Json.Decode.succeed b

                Err err ->
                    Json.Decode.fail <| Json.Decode.errorToString err
        )
        decoder


optionalField : String -> (a -> Json.Encode.Value) -> Maybe a -> Maybe ( String, Json.Encode.Value )
optionalField field encoder =
    Maybe.map (\val -> ( field, encoder val ))

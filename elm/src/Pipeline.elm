port module Pipeline exposing (Model, Msg(..), Flags, init, update, view, subscriptions, loadPipeline)

import Html exposing (Html)
import Html.Attributes exposing (class, href, id, style, src, width, height)
import Html.Attributes.Aria exposing (ariaLabel)
import Http
import Json.Encode
import Json.Decode
import Navigation
import Svg exposing (..)
import Svg.Attributes as SvgAttributes
import Task
import Time exposing (Time)

import Concourse
import Concourse.Cli
import Concourse.Info
import Concourse.Job
import Concourse.Resource
import Concourse.Pipeline
import Route.QueryString as QueryString
import Routes

type alias Ports =
  { render : (Json.Encode.Value, Json.Encode.Value) -> Cmd Msg
  , title : String -> Cmd Msg
  }

type alias Model =
  { ports : Ports
  , pipelineLocator : Concourse.PipelineIdentifier
  , fetchedJobs : Maybe Json.Encode.Value
  , fetchedResources : Maybe Json.Encode.Value
  , renderedJobs : Maybe Json.Encode.Value
  , renderedResources : Maybe Json.Encode.Value
  , concourseVersion : String
  , turbulenceImgSrc : String
  , experiencingTurbulence : Bool
  , selectedGroups : List String
  }

type alias Flags =
  { teamName : String
  , pipelineName : String
  , turbulenceImgSrc : String
  , route : Routes.ConcourseRoute
  }

type Msg
  = Noop
  | AutoupdateVersionTicked Time
  | AutoupdateTimerTicked Time
  | PipelineIdentifierFetched Concourse.PipelineIdentifier
  | JobsFetched (Result Http.Error Json.Encode.Value)
  | ResourcesFetched (Result Http.Error Json.Encode.Value)
  | VersionFetched (Result Http.Error String)
  | PipelineFetched (Result Http.Error Concourse.Pipeline)

queryGroupsForRoute : Routes.ConcourseRoute -> List String
queryGroupsForRoute route =
  QueryString.all "groups" route.queries

init : Ports -> Flags -> (Model, Cmd Msg)
init ports flags =
  let
    pipelineLocator =
      { teamName = flags.teamName
      , pipelineName = flags.pipelineName
      }
    model =
      { ports = ports
      , concourseVersion = ""
      , turbulenceImgSrc = flags.turbulenceImgSrc
      , pipelineLocator = pipelineLocator
      , fetchedJobs = Nothing
      , fetchedResources = Nothing
      , renderedJobs = Nothing
      , renderedResources = Nothing
      , experiencingTurbulence = False
      , selectedGroups = queryGroupsForRoute flags.route
      }
  in
    loadPipeline pipelineLocator model

loadPipeline : Concourse.PipelineIdentifier -> Model -> (Model, Cmd Msg)
loadPipeline pipelineLocator model =
  ( { model
    | pipelineLocator = pipelineLocator
    }
  , Cmd.batch
      [ fetchPipeline pipelineLocator
      , fetchVersion
      , model.ports.title <| model.pipelineLocator.pipelineName ++ " - "
      ]
  )

update : Msg -> Model -> (Model, Cmd Msg)
update msg model =
  case msg of
    Noop ->
      (model, Cmd.none)

    AutoupdateTimerTicked timestamp ->
      ( model
      , fetchPipeline model.pipelineLocator
      )

    PipelineIdentifierFetched pipelineIdentifier ->
      (model, fetchPipeline pipelineIdentifier)

    AutoupdateVersionTicked _ ->
      (model, fetchVersion)

    PipelineFetched (Ok pipeline) ->
      let
        firstGroup =
          List.head pipeline.groups
        groups =
          if List.isEmpty model.selectedGroups then
            case firstGroup of
              Nothing ->
                []
              Just group ->
                [group.name]
          else
            model.selectedGroups
      in
        Debug.log "PipelineFetched" ( { model
          | selectedGroups = groups
          }
        , Cmd.batch
          [ fetchJobs model.pipelineLocator
          , fetchResources model.pipelineLocator
          ]
        )

    PipelineFetched (Err (Http.BadResponse 401 _)) ->
      (model, Navigation.newUrl "/login")

    PipelineFetched (Err err) ->
      renderIfNeeded { model | experiencingTurbulence = True }

    JobsFetched (Ok fetchedJobs) ->
      renderIfNeeded { model | fetchedJobs = Just fetchedJobs, experiencingTurbulence = False }

    JobsFetched (Err (Http.BadResponse 401 _)) ->
      (model, Navigation.newUrl "/login")

    JobsFetched (Err err) ->
      renderIfNeeded { model | fetchedJobs = Nothing, experiencingTurbulence = True }

    ResourcesFetched (Ok fetchedResources) ->
      Debug.log "ResourcesFetched"
      renderIfNeeded { model | fetchedResources = Just fetchedResources, experiencingTurbulence = False }

    ResourcesFetched (Err (Http.BadResponse 401 _)) ->
      (model, Navigation.newUrl "/login")

    ResourcesFetched (Err err) ->
      renderIfNeeded { model | fetchedResources = Nothing, experiencingTurbulence = True }

    VersionFetched (Ok version) ->
      ({ model | concourseVersion = version, experiencingTurbulence = False }, Cmd.none)

    VersionFetched (Err err) ->
      Debug.log ("failed to fetch version: " ++ toString err) <|
        ({ model | experiencingTurbulence = True }, Cmd.none)

subscriptions : Model -> Sub Msg
subscriptions model =
  Sub.batch
    [ autoupdateVersionTimer
    , Time.every (5 * Time.second) AutoupdateTimerTicked
    ]

view : Model -> Html Msg
view model =
  Html.div []
    [ Svg.svg
      [ SvgAttributes.class "pipeline-graph test"
      , SvgAttributes.width "100%"
      , SvgAttributes.height "100%"
      ] []
    , Html.div [if model.experiencingTurbulence then class "error-message" else class "error-message hidden"]
        [ Html.div [class "message"]
            [ Html.img [src model.turbulenceImgSrc, class "seatbelt"] []
            , Html.p [] [Html.text "experiencing turbulence"]
            , Html.p [class "explanation"] []
            ]
        ]
    , Html.dl [class "legend"]
            [ Html.dt [class "pending"] []
            , Html.dd [] [Html.text "pending"]
            , Html.dt [class "started"] []
            , Html.dd [] [Html.text "started"]
            , Html.dt [class "succeeded"] []
            , Html.dd [] [Html.text "succeeded"]
            , Html.dt [class "failed"] []
            , Html.dd [] [Html.text "failed"]
            , Html.dt [class "errored"] []
            , Html.dd [] [Html.text "errored"]
            , Html.dt [class "aborted"] []
            , Html.dd [] [Html.text "aborted"]
            , Html.dt [class "paused"] []
            , Html.dd [] [Html.text "paused"]
            ]
        , Html.table [class "lower-right-info"]
            [ Html.tr []
                [ Html.td [class "label"] [ Html.text "cli:"]
                , Html.td []
                    [ Html.ul [class "cli-downloads"]
                        [ Html.li []
                            [ Html.a
                                [href (Concourse.Cli.downloadUrl "amd64" "darwin"), ariaLabel "Download OS X CLI"]
                                [ Html.i [class "fa fa-apple"] [] ]
                            ]
                        , Html.li []
                            [ Html.a
                                [href (Concourse.Cli.downloadUrl "amd64" "windows"), ariaLabel "Download Windows CLI"]
                                [ Html.i [class "fa fa-windows"] [] ]
                            ]
                        , Html.li []
                            [ Html.a
                                [href (Concourse.Cli.downloadUrl "amd64" "linux"), ariaLabel "Download Linux CLI"]
                                [ Html.i [class "fa fa-linux"] [] ]
                            ]
                        ]
                    ]
                ]
            , Html.tr []
                [ Html.td [class "label"] [ Html.text "version:" ]
                , Html.td []
                    [ Html.div [id "concourse-version"]
                        [ Html.text "v"
                        , Html.span [class "number"] [Html.text model.concourseVersion]
                        ]
                    ]
                ]
            ]
    ]

autoupdateVersionTimer : Sub Msg
autoupdateVersionTimer =
  Time.every (1 * Time.minute) AutoupdateVersionTicked

jobAppearsInGroups : List String -> Concourse.PipelineIdentifier -> Json.Encode.Value -> Bool
jobAppearsInGroups groupNames pi jobJson =
  let concourseJob =
    Json.Decode.decodeValue (Concourse.decodeJob pi) jobJson
  in
    case concourseJob of
      Ok cj ->
        anyIntersect cj.groups groupNames
      Err err ->
        Debug.log ("failed to check if job is in group: " ++ toString err) False

expandJsonList : Json.Encode.Value -> List Json.Decode.Value
expandJsonList flatList =
  let
    result =
      Json.Decode.decodeValue (Json.Decode.list Json.Decode.value) flatList
  in
    case result of
      Ok res ->
        res
      Err err ->
        []

filterJobs : Model -> Json.Encode.Value -> Json.Encode.Value
filterJobs model value =
  Json.Encode.list <|
    List.filter
      (jobAppearsInGroups model.selectedGroups model.pipelineLocator)
      (expandJsonList value)

renderIfNeeded : Model ->  (Model, Cmd Msg)
renderIfNeeded model =



  case (model.fetchedResources, model.fetchedJobs) of
    (Just fetchedResources, Just fetchedJobs) ->
      let
        filteredFetchedJobs =
          if List.isEmpty model.selectedGroups then
            fetchedJobs
          else
            filterJobs model fetchedJobs
      in
        case (model.renderedResources, model.renderedJobs) of
          (Just renderedResources, Just renderedJobs) ->
            if (expandJsonList renderedJobs /= expandJsonList filteredFetchedJobs)
              || (expandJsonList renderedResources /= expandJsonList fetchedResources) then
                ( { model
                  | renderedJobs = Just filteredFetchedJobs
                  , renderedResources = Just fetchedResources
                  }
                , model.ports.render (filteredFetchedJobs, fetchedResources)
                )
            else
              (model, Cmd.none)
          _ ->
            ( { model
              | renderedJobs = Just filteredFetchedJobs
              , renderedResources = Just fetchedResources
              }
            , model.ports.render (filteredFetchedJobs, fetchedResources)
            )
    _ ->
      (model, Cmd.none)

fetchResources : Concourse.PipelineIdentifier -> Cmd Msg
fetchResources pid =
  Cmd.map ResourcesFetched << Task.perform Err Ok <| Concourse.Resource.fetchResourcesRaw pid

fetchJobs : Concourse.PipelineIdentifier -> Cmd Msg
fetchJobs pid =
  Cmd.map JobsFetched << Task.perform Err Ok <| Concourse.Job.fetchJobsRaw pid

fetchVersion : Cmd Msg
fetchVersion =
  Concourse.Info.fetchVersion
    |> Task.perform Err Ok
    |> Cmd.map VersionFetched

fetchPipeline : Concourse.PipelineIdentifier -> Cmd Msg
fetchPipeline pipelineIdentifier =
  Cmd.map PipelineFetched <|
    Task.perform Err Ok (Concourse.Pipeline.fetchPipeline pipelineIdentifier)

anyIntersect : List a -> List a -> Bool
anyIntersect list1 list2 =
  case list1 of
    [] -> False
    first :: rest ->
      if List.member first list2 then True
      else anyIntersect rest list2

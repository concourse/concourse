module Dashboard.Models exposing
    ( DragState(..)
    , DropState(..)
    , Dropdown(..)
    , FetchError(..)
    , FooterModel
    , Model
    )

import Concourse
import Dashboard.Group.Models
import Dict exposing (Dict)
import FetchResult exposing (FetchResult)
import Login.Login as Login
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DropTarget)
import Routes
import Set exposing (Set)
import Time


type alias Model =
    FooterModel
        (Login.Model
            { now : Maybe Time.Posix
            , highDensity : Bool
            , query : String
            , pipelinesWithResourceErrors : Set Concourse.DatabaseID
            , jobs : FetchResult (Dict ( Concourse.DatabaseID, String ) Concourse.Job)
            , pipelineLayers : Dict Concourse.DatabaseID (List (List Concourse.JobIdentifier))
            , teams : FetchResult (List Concourse.Team)
            , dragState : DragState
            , dropState : DropState
            , isJobsRequestFinished : Bool
            , isTeamsRequestFinished : Bool
            , isPipelinesRequestFinished : Bool
            , isResourcesRequestFinished : Bool
            , jobsError : Maybe FetchError
            , teamsError : Maybe FetchError
            , resourcesError : Maybe FetchError
            , pipelinesError : Maybe FetchError
            , viewportWidth : Float
            , viewportHeight : Float
            , scrollTop : Float
            , pipelineJobs : Dict Concourse.DatabaseID (List Concourse.JobIdentifier)
            , effectsToRetry : List Effect
            }
        )


type FetchError
    = Failed
    | Disabled


type DragState
    = NotDragging
    | Dragging Concourse.TeamName Concourse.PipelineName


type DropState
    = NotDropping
    | Dropping DropTarget
    | DroppingWhileApiRequestInFlight Concourse.TeamName


type alias FooterModel r =
    { r
        | hideFooter : Bool
        , hideFooterCounter : Int
        , showHelp : Bool
        , pipelines : Maybe (Dict String (List Dashboard.Group.Models.Pipeline))
        , dropdown : Dropdown
        , highDensity : Bool
        , dashboardView : Routes.DashboardView
    }


type Dropdown
    = Hidden
    | Shown (Maybe Int)

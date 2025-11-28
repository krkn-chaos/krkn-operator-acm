# Overview 
this is an operator used to integrate with RedHat ACM (Advanced Cluster Management) and a custom made operator
called krkn-operator.

# Requirements

- the operator must react to the creation of a CRD called `KrknTargetRequest`, this CRD contains a UUID, a targetData field
  (that will be a json that I'll describe later) and a status field
- the `KrknTargetRequest` CR is created by another operator with a valorized UUID and status to `pending`

once the operator receives the `KrknTargetRequest` the operator must:

- make an api request to  /apis/cluster.open-cluster-management.io/v1/managedclusters, the response payload can be found
  in misc/managed-cluster.json. This Api request goal is to list and collect the available 
  - cluster names (jsonpath .items[].name)
  - cluster API URL (jsonpath .items[].spec.managedClusterClientConfigs[].url)
  - cluster CA Bundle (jsonpath .items[].spec.managedClusterClientConfigs[].caBunble)
  - Each cluster name will have a namespace with the same name of the cluster containing a secret called `application-manager`
    - `application-manager` secret has two fields base64 encoded
      - ca.crt (containing the ca certificate of the cluster api)
      - token (the token that is used to query the managed cluster API)
- with all the information gathered must build a valid kubeconfig per each cluster
- create a secret which name is the UUID of the request
  - the secret must have a field .data.managed-clusters
    - the data must be a json map key with the name of the cluster and value an object 
    - the object must contain the cluster-name, cluster-api and kubeconfig (base64 encoded)
- when the secret is created the `KrknTargetRequest` status must be changed to Completed and the field targetData will contain a list of:
  - cluster-name
  - cluster-api-url

# Refactoring 28/11

- I want to add a New CRD called KrknOperatorTargetProvider, the goal is to register the operator with its operator-name and a timestamp
  At the operator boot, if the CR already exists with the same operator-name only the timestamp must be updated
- The operator name must come from a config-map this, configmap will progressively contain many other options so must be reachable
  from almost any place in the reconcile loop 
- I want to add two timestamp to KrknTargetRequest one is `created` the other one is `completed` and they must be valorized
  respectively when the CR is created and one when it's completed
- I want to change both the CRD KrknTargetRequestStatus , the attribute TargetData []ClusterTarget must become a map[str][]ClusterTarget
  where the key is the operator-name and the value are the targets associated with the operator itself, the aim is to allow multiple
  operator with different data sources might set values on the same CR
- I want to make the same change also in the json data structure in the Secret
- on every reconcile loop the operator must list the number of 
- 
- I want that in the reconcile loop the status of the CR must be completed when the number of keys in the map[str][]ClusterTarget
  
- equals the number of KrknOperatorTargetProvider listed before

# Refactoring 2 28/11

- I want to add a field active bool al CRD KrknOperatorTargetProvider
- I want the reconcile loop to count only the target provider that are Active == true
- I want to add the operator namespace in the global configmap
- I want to create a config object that is mapped in a methdo from the values contained in the configmap and returned from
  that method, and I want to remove the single property collection methods like `getOperatorName`
- check  the krkntargetrequest_controller.go line 133 `Requeue` is deprecated, find a solution.


 


  
